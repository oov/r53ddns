package main

import (
	"encoding/json"
	"errors"
	"flag"
	"github.com/crowdmob/goamz/aws"
	"github.com/oov/r53ddns/route53"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"
)

type Store struct {
	IP string
}

var (
	zoneID     = flag.String("zone", "", "hosted zone id")
	domain     = flag.String("domain", "", "domain ex: ddns.example.org.")
	ttl        = flag.Int("ttl", 600, "TTL")
	delay      = flag.Int("delay", 20, "how long to wait before GetChange")
	ipDetector = flag.String("ip", "http://checkip.dyndns.org/", "public ip address detector")
)

func load(path string) (*Store, error) {
	var stored Store
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		defer f.Close()
		err = json.NewDecoder(f).Decode(&stored)
		if err != nil {
			return nil, err
		}
	}
	return &stored, nil
}

func save(path string, s *Store) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s)
}

func discoverPublicIP() (string, error) {
	r, err := http.Get(*ipDetector)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	found := regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+`).Find(buf)
	ip := net.ParseIP(string(found))
	if ip == nil {
		return "", errors.New("could not detect valid ipv4 address")
	}
	return ip.String(), nil
}

func main() {
	flag.Parse()

	if *zoneID == "" || *domain == "" {
		log.Println("'zone' and 'domain' are required.")
		flag.PrintDefaults()
		return
	}

	storedPath := path.Join(os.TempDir(), "r53ddns-"+*zoneID+"-"+*domain+".json")
	stored, err := load(storedPath)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("latest public ip is:", stored.IP)

	log.Println("discovering my public ip...")
	publicIP, err := discoverPublicIP()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("public ip is:", publicIP)

	if stored.IP == publicIP {
		log.Println("no changed found.")
		return
	}

	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatalln(err)
	}
	r53 := route53.New(auth)

	log.Println("route53: call ListResourceRecordSets")
	lrrsr, err := r53.ListResourceRecordSets(*zoneID, map[string]string{
		"name":     *domain,
		"type":     "A",
		"maxitems": "1",
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("route53: call ChangeResourceRecordSets")
	crrsr, err := r53.ChangeResourceRecordSets(*zoneID, []route53.ChangeResourceRecord{
		{
			Action: "DELETE",
			RRSet:  lrrsr.RRSets[0],
		},
		{
			Action: "CREATE",
			RRSet: route53.ResourceRecordSet{
				Name:  *domain,
				Type:  "A",
				TTL:   *ttl,
				Value: publicIP,
			},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}
	if crrsr.Status != "PENDING" {
		log.Fatalln("status is not PENDING:", crrsr.Status)
	}

	if crrsr.Id[:8] == "/change/" {
		crrsr.Id = crrsr.Id[8:]
	}

	var success bool
	for tries := 0; tries < 5; tries++ {
		log.Println("change ID:", crrsr.Id, "waiting...")
		time.Sleep(time.Duration(*delay) * time.Second)

		log.Println("route53: call GetChange")
		gcr, err := r53.GetChange(crrsr.Id)
		if err != nil {
			log.Fatalln(err)
		}

		if gcr.Status == "INSYNC" {
			success = true
			break
		}
		log.Println("not completed yet")
	}
	if !success {
		log.Fatalln("could not complete")
	}

	log.Println("update completed")

	stored.IP = publicIP
	err = save(storedPath, stored)
	if err != nil {
		log.Fatalln(err)
	}
}

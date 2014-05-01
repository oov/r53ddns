package main

import (
	"encoding/json"
	"flag"
	"github.com/ccding/go-stun/stun"
	"github.com/crowdmob/goamz/aws"
	"log"
	"os"
	"path"
	"time"
)

type Store struct {
	IP string
}

var (
	zoneID = flag.String("zone", "", "hosted zone id")
	domain = flag.String("domain", "", "domain ex: ddns.example.org.")
	ttl    = flag.Int("ttl", 600, "TTL")
	delay  = flag.Int("delay", 20, "how long to wait before GetChange")
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
	log.Println("latest global ip is:", stored.IP)

	log.Println("discovering my global ip...")
	_, host, err := stun.Discover()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("global ip is:", host.Ip())

	if stored.IP == host.Ip() {
		log.Println("no changed found.")
		return
	}

	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatalln(err)
	}
	r53 := NewRoute53(auth)

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
	crrsr, err := r53.ChangeResourceRecordSets(*zoneID, []ChangeResourceRecord{
		{
			Action: "DELETE",
			RRSet:  lrrsr.RRSets[0],
		},
		{
			Action: "CREATE",
			RRSet: ResourceRecordSet{
				Name:  *domain,
				Type:  "A",
				TTL:   *ttl,
				Value: host.Ip(),
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

	stored.IP = host.Ip()
	err = save(storedPath, stored)
	if err != nil {
		log.Fatalln(err)
	}
}

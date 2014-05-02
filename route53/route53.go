package route53

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/crowdmob/goamz/aws"
	"io"
	"net/http"
	"net/url"
)

type ResourceRecordSet struct {
	XMLName       xml.Name `xml:"ResourceRecordSet"`
	Name          string   `xml:"Name"`
	Type          string   `xml:"Type"`
	TTL           int      `xml:"TTL,omitempty"`
	Value         string   `xml:"ResourceRecords>ResourceRecord>Value"`
	HealthCheckId string   `xml:"HealthCheckId,omitempty"`
}

type ChangeResourceRecordSets struct {
	XMLName xml.Name               `xml:"ChangeResourceRecordSetsRequest"`
	Xmlns   string                 `xml:"xmlns,attr"`
	Comment string                 `xml:"ChangeBatch>Comment,omitempty"`
	Changes []ChangeResourceRecord `xml:"ChangeBatch>Changes>Change"`
}

type ChangeResourceRecord struct {
	Action string `xml:"Action"`
	RRSet  ResourceRecordSet
}

type ChangeResourceRecordSetsResponse struct {
	XMLName     xml.Name `xml:"ChangeResourceRecordSetsResponse"`
	Id          string   `xml:"ChangeInfo>Id"`
	Status      string   `xml:"ChangeInfo>Status"`
	SubmittedAt string   `xml:"ChangeInfo>SubmittedAt"`
}

type ListResourceRecordSetsResponse struct {
	XMLName              xml.Name            `xml:"ListResourceRecordSetsResponse"`
	RRSets               []ResourceRecordSet `xml:"ResourceRecordSets>ResourceRecordSet"`
	IsTruncated          bool                `xml:"IsTruncated"`
	MaxItems             int                 `xml:"MaxItems"`
	NextRecordType       string              `xml:"NextRecordType"`
	NextRecordIdentifier string              `xml:"NextRecordIdentifier"`
}

type GetChangeResponse struct {
	XMLName     xml.Name `xml:"GetChangeResponse"`
	Id          string   `xml:"ChangeInfo>Id"`
	Status      string   `xml:"ChangeInfo>Status"`
	SubmittedAt string   `xml:"ChangeInfo>SubmittedAt"`
}

const route53API = "https://route53.amazonaws.com/2013-04-01"
const route53Doc = "https://route53.amazonaws.com/doc/2013-04-01/"

type Route53 struct {
	signer *aws.Route53Signer
}

func New(auth aws.Auth) *Route53 {
	return &Route53{
		signer: aws.NewRoute53Signer(auth),
	}
}

func (r53 Route53) Query(method string, endpoint string, body io.Reader, result interface{}) error {
	r, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}

	r53.signer.Sign(r)
	client := &http.Client{}
	res, err := client.Do(r)
	defer res.Body.Close()
	if err != nil {
		return err
	}

	if res.StatusCode != 201 && res.StatusCode != 200 {
		return (&aws.Service{}).BuildError(res)
	}

	return xml.NewDecoder(res.Body).Decode(result)
}

func (r53 Route53) ListResourceRecordSets(zoneID string, params map[string]string) (*ListResourceRecordSetsResponse, error) {
	url, err := url.Parse(fmt.Sprintf("%s/hostedzone/%s/rrset", route53API, zoneID))
	q := url.Query()
	if params != nil {
		for k, v := range params {
			q.Set(k, v)
		}
	}
	url.RawQuery = q.Encode()
	var r ListResourceRecordSetsResponse
	err = r53.Query("GET", url.String(), nil, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (r53 Route53) ChangeResourceRecordSets(zoneID string, Changes []ChangeResourceRecord) (*ChangeResourceRecordSetsResponse, error) {
	req := ChangeResourceRecordSets{
		Xmlns:   route53Doc,
		Changes: Changes,
	}

	b, err := xml.Marshal(req)
	if err != nil {
		return nil, err
	}

	var r ChangeResourceRecordSetsResponse
	err = r53.Query("POST", fmt.Sprintf("%s/hostedzone/%s/rrset", route53API, zoneID), bytes.NewBuffer(b), &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

func (r53 Route53) GetChange(changeID string) (*GetChangeResponse, error) {
	var r GetChangeResponse
	err := r53.Query("GET", fmt.Sprintf("%s/change/%s", route53API, changeID), nil, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

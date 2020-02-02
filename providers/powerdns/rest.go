package powerdns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"
)

const DefaultServerId string = "localhost" /* https://doc.powerdns.com/authoritative/http-api/server.html */
const (
	ChangeTypeReplace = "REPLACE"
	ChangeTypeDelete  = "DELETE"
)

type PowerDnsApiClient struct {
	client  *http.Client
	apiKey  string
	baseUrl string
}

type PdnsComments struct {
	Content    string `json:"content,omitempty"`
	Account    string `json:"account,omitempty"`
	ModifiedAt int    `json:"modified_at,omitempty"`
}

type PdnsRecord struct {
	Content  string `json:"content,omitempty"`
	Disabled bool   `json:"disabled"`
	SetPr    bool   `json:"set-ptr"`
}

type PdnsRRSet struct {
	Name       string         `json:"name,omitempty"`
	Type       string         `json:"type,omitempty"`
	TTL        int            `json:"ttl,omitempty"`
	ChangeType string         `json:"changetype,omitempty"` // REPLACE or DELETE
	Records    []PdnsRecord   `json:"records,omitempty"`
	Comments   []PdnsComments `json:"comments,omitempty"`
}

type PdnsZone struct {
	Id               string      `json:"id,omitempty"`
	Name             string      `json:"name,omitempty"`
	Type             string      `json:"type,omitempty"`
	Url              string      `json:"url,omitempty"`
	Kind             string      `json:"kind,omitempty"`
	RRsets           []PdnsRRSet `json:"rrsets,omitempty"`
	Serial           int         `json:"serial,omitempty"`
	NotifiedSerial   int         `json:"notified_serial,omitempty"`
	EditedSerial     int         `json:"edited_serial,omitempty"`
	Masters          []string    `json:"masters,omitempty"`
	DnsSec           bool        `json:"dnssec"`
	Nsec3param       string      `json:"nsec3param,omitempty"`
	Nsec3narrow      bool        `json:"nsec3narrow"`
	Presigned        bool        `json:"presigned"`
	SoaEdit          string      `json:"soa_edit,omitempty"`
	SoaEditApi       string      `json:"soa_edit_api,omitempty"`
	ApiRectify       bool        `json:"api_rectify"`
	Zone             string      `json:"zone,omitempty"`
	Account          string      `json:"account,omitempty"`
	Nameservers      []string    `json:"nameservers,omitempty"`
	MasterTsigKeyIds []string    `json:"master_tsig_key_ids,omitempty"`
	SlaveTsigKeyIds  []string    `json:"slave_tsig_key_ids,omitempty"`
}

type RRSetsUpdate struct {
	RRSets []PdnsRRSet `json:"rrsets"`
}

func NewPowerDnsApiClient(apiKey, baseUrl string) PowerDnsApiClient {
	return PowerDnsApiClient{
		apiKey:  apiKey,
		baseUrl: baseUrl,
		client: &http.Client{
			Timeout: time.Second * 20,
		},
	}
}

func (c *PowerDnsApiClient) getApiUrl() (*url.URL, error) {
	u, err := url.Parse(c.baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "api/v1")
	return u, nil
}

func (c *PowerDnsApiClient) createRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	req.Header.Add("X-API-Key", c.apiKey)
	req.Header.Add("Accept", "application/json")
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *PowerDnsApiClient) GetZones() (*[]PdnsZone, error) {
	reqUrl, err := c.getApiUrl()
	if err != nil {
		return nil, err
	}
	reqUrl.Path = path.Join(reqUrl.Path, "/servers","/", DefaultServerId, "/zones")

	req, err := c.createRequest("GET", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get DNS zones: %s", response.Status)
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	zones := new([]PdnsZone)
	err = json.Unmarshal(responseBytes, &zones)
	if err != nil {
		return nil, err
	}

	return zones, nil
}

func (c *PowerDnsApiClient) GetZone(zoneId string) (*PdnsZone, error) {
	reqUrl, err := c.getApiUrl()
	if err != nil {
		return nil, err
	}
	reqUrl.Path = path.Join(reqUrl.Path, "/servers","/", DefaultServerId, "/zones", "/", zoneId)

	req, err := c.createRequest("GET", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("failed to get DNS zone: %s", response.Status)
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("domain %s does not exists in DNS zone", zoneId)
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	zones := &PdnsZone{}
	err = json.Unmarshal(responseBytes, &zones)
	if err != nil {
		return nil, err
	}

	return zones, nil
}


func (c *PowerDnsApiClient) GetZoneRRSets(zoneId string) ([]PdnsRRSet, error) {
	reqUrl, err := c.getApiUrl()
	if err != nil {
		return nil, err
	}
	reqUrl.Path = path.Join(reqUrl.Path, "/servers","/", DefaultServerId, "/zones", "/", zoneId)


	req, err := c.createRequest("GET", reqUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get DNS zones RRSets: %s", response.Status)
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	zone := PdnsZone{}
	err = json.Unmarshal(responseBytes, &zone)
	if err != nil {
		return nil, err
	}

	return zone.RRsets, nil
}

func (c *PowerDnsApiClient) UpdateZoneRRSets(zoneId string, rrSets []PdnsRRSet) (error) {
	reqUrl, err := c.getApiUrl()
	if err != nil {
		return err
	}
	reqUrl.Path = path.Join(reqUrl.Path, "/servers","/", DefaultServerId, "/zones", "/", zoneId)

	updateRequest := RRSetsUpdate{
		RRSets: rrSets,
	}

	jsonBytes, err := json.Marshal(updateRequest)
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))

	reader := bytes.NewReader(jsonBytes)
	req, err := c.createRequest("PATCH", reqUrl.String(), reader)
	if err != nil {
		return err
	}

	response, err := c.client.Do(req)
	if err != nil {
		return err
	}

	responseBytes, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(responseBytes))
	
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to update DNS records %s", response.Status)
	}

	return nil
}

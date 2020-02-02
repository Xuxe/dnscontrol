package powerdns

import (
	"encoding/json"
	"fmt"
	"github.com/StackExchange/dnscontrol/v2/models"
	"github.com/StackExchange/dnscontrol/v2/providers"
	"github.com/StackExchange/dnscontrol/v2/providers/diff"
)

func init() {
	providers.RegisterDomainServiceProviderType("POWERDNS", newPowerDNSProvider)
}

type PowerDNSProvider struct {
	apiClient PowerDnsApiClient
}

func newPowerDNSProvider(m map[string]string, metadata json.RawMessage) (providers.DNSServiceProvider, error) {
	if m["apikey"] == "" || m["baseurl"] == "" {
		return nil, fmt.Errorf("PowerDNS Provider: Api key and/or base url missing. You maybe forgot to setup creds.json?")
	}

	apiClient := NewPowerDnsApiClient(m["apikey"], m["baseurl"])
	_, err := apiClient.GetZones()
	if err != nil {
		return nil, err
	}

	provider := &PowerDNSProvider{
		apiClient: apiClient,
	}

	return provider, nil
}

func (p *PowerDNSProvider) GetNameservers(domain string) ([]*models.Nameserver, error) {
	zone, err := p.apiClient.GetZone(domain)
	if err != nil {
		return nil, err
	}

	if zone == nil {
		return nil, fmt.Errorf("could not get nameservers zone not found")
	}

	var ns []string

	for _, v := range zone.RRsets {
		if v.Type == "NS" {
			for _, x := range v.Records {
				ns = append(ns, x.Content)
			}
		}
	}
	return models.StringsToNameservers(ns), nil
}

func (p *PowerDNSProvider) GetDomainCorrections(dc *models.DomainConfig) ([]*models.Correction, error) {
	dc.Punycode()

	zone, err := p.apiClient.GetZone(dc.Name)
	if err != nil {
		return nil, err
	}
	currentRecords := p.nativeToDomainConfig(zone.RRsets, dc)
	models.PostProcessRecords(currentRecords)

	differ := diff.New(dc)
	_, create, del, modify := differ.IncrementalDiff(currentRecords)

	var corrections = []*models.Correction{}

	for _, d := range del {
	   c := p.buildCorrection(d, dc, "delete")
	   corrections = append(corrections, c)
	}

	for _, m := range modify {
		c := p.buildCorrection(m, dc, "modify")
		corrections = append(corrections, c)
	}

	for _, c := range create {
		c := p.buildCorrection(c, dc, "create")
		corrections = append(corrections, c)
	}

	return corrections, nil
}

func (p *PowerDNSProvider) buildCorrection(c diff.Correlation, dc *models.DomainConfig, action string) *models.Correction{
	if action == "create" || action == "modify" {
		correction := models.Correction{
			Msg: fmt.Sprintf("%s", c.String()),
			F: func() error {
				set := PdnsRRSet{}
				sets := make([]PdnsRRSet, 1)
				set.ChangeType = ChangeTypeReplace
				set.Name = fmt.Sprintf("%s.",c.Desired.GetLabelFQDN())
				set.Type = c.Desired.Type
				set.Records = make([]PdnsRecord, 1)
				set.Records[0] = PdnsRecord{
					 Content: c.Desired.GetTargetField(),
					 Disabled: false,
				}
				set.TTL = int(c.Desired.TTL)
				sets[0] = set
				return p.apiClient.UpdateZoneRRSets(dc.Name, sets)
			},
		}
		return &correction
	} else {
		correction := models.Correction{
			Msg: fmt.Sprintf("%s", c.String()),
			F: func() error {
				set := PdnsRRSet{}
				sets := make([]PdnsRRSet, 1)
				set.ChangeType = ChangeTypeDelete
				set.Name = fmt.Sprintf("%s.", c.Existing.GetLabelFQDN())
				set.Type = c.Existing.Type
				set.Records = make([]PdnsRecord, 1)
				set.Records[0] = PdnsRecord{
				 	Content: c.Existing.GetTargetField(),
				}
				sets[0] = set
				return p.apiClient.UpdateZoneRRSets(dc.Name, sets)
			},
		}
		return &correction
	}
}

func (p *PowerDNSProvider) nativeToDomainConfig(native []PdnsRRSet, dc *models.DomainConfig) []*models.RecordConfig {
	config := make([]*models.RecordConfig, 0)
	for _, r := range native {
		for _, rr := range r.Records {
			rcc := &models.RecordConfig{
				Type:   r.Type,
				Target: rr.Content,
				TTL:    uint32(r.TTL),
			}
			rcc.SetLabelFromFQDN(r.Name, dc.Name)
			config = append(config, rcc)
		}
	}
	return config
}

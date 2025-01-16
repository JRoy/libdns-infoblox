package infoblox

import (
	"context"
	"fmt"
	ibclient "github.com/infobloxopen/infoblox-go-client/v2"
	"github.com/libdns/libdns"
)

// Provider facilitates DNS record manipulation with Infoblox
type Provider struct {
	Host     string `json:"host,omitempty"`
	Version  string `json:"version,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(_ context.Context, zone string) ([]libdns.Record, error) {
	conn, err := p.getConnector()
	if err != nil {
		return nil, fmt.Errorf("failed to get connector: %w", err)
	}
	qp := ibclient.NewQueryParams(false, map[string]string{"name": zone})

	var cnameRecords []ibclient.RecordCNAME
	err = conn.GetObject(&ibclient.RecordCNAME{}, "", qp, &cnameRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to get CNAME records: %w", err)
	}

	var txtRecords []ibclient.RecordTXT
	err = conn.GetObject(&ibclient.RecordTXT{}, "", qp, &txtRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to get TXT records: %w", err)
	}

	var list []libdns.Record
	for i := range cnameRecords {
		list = append(list, libdns.Record{
			Type:  "CNAME",
			Name:  *cnameRecords[i].Name,
			Value: *cnameRecords[i].Canonical,
		})
	}

	for i := range txtRecords {
		list = append(list, libdns.Record{
			Type:  "TXT",
			Name:  *txtRecords[i].Name,
			Value: *txtRecords[i].Text,
		})
	}

	return list, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var added []libdns.Record

	objMgr, err := p.getObjectManager()
	if err != nil {
		return nil, fmt.Errorf("failed to get object manager: %w", err)
	}

	for i := range records {
		switch records[i].Type {
		case "CNAME":
			record, err := objMgr.CreateCNAMERecord("default", records[i].Value, records[i].Name, true, uint32(records[i].TTL.Seconds()), "", nil)
			if err != nil {
				continue
			}
			added = append(added, libdns.Record{
				Type:  "CNAME",
				Name:  *record.Name,
				Value: *record.Canonical,
			})
		case "TXT":
			record, err := objMgr.CreateTXTRecord("default", records[i].Name, records[i].Value, uint32(records[i].TTL.Seconds()), true, "", nil)
			if err != nil {
				continue
			}
			added = append(added, libdns.Record{
				Type:  "TXT",
				Name:  *record.Name,
				Value: *record.Text,
			})
		}
	}

	return added, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var updated []libdns.Record

	objMgr, err := p.getObjectManager()
	if err != nil {
		return nil, fmt.Errorf("failed to get object manager: %w", err)
	}

	for i := range records {
		switch records[i].Type {
		case "CNAME":
			record, err := objMgr.GetCNAMERecord("default", "", records[i].Name)
			if err != nil {
				record, err = objMgr.CreateCNAMERecord("default", records[i].Value, records[i].Name, true, uint32(records[i].TTL.Seconds()), "", nil)
				if err != nil {
					continue
				}
			} else {
				_, err := objMgr.UpdateCNAMERecord(record.Ref, records[i].Value, *record.Name, *record.UseTtl, *record.Ttl, *record.Comment, record.Ea)
				if err != nil {
					continue
				}
			}
			updated = append(updated, libdns.Record{
				Type:  "CNAME",
				Name:  *record.Name,
				Value: *record.Canonical,
			})
		case "TXT":
			record, err := objMgr.GetTXTRecord("default", records[i].Name)
			if err != nil {
				record, err = objMgr.CreateTXTRecord("default", records[i].Name, records[i].Value, uint32(records[i].TTL.Seconds()), true, "", nil)
				if err != nil {
					continue
				}
			} else {
				record, err = objMgr.UpdateTXTRecord(record.Ref, *record.Name, records[i].Value, *record.Ttl, *record.UseTtl, *record.Comment, record.Ea)
				if err != nil {
					continue
				}
			}
			updated = append(updated, libdns.Record{
				Type:  "TXT",
				Name:  *record.Name,
				Value: *record.Text,
			})
		}
	}

	return updated, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var deleted []libdns.Record

	objMgr, err := p.getObjectManager()
	if err != nil {
		return nil, fmt.Errorf("failed to get object manager: %w", err)
	}

	for i := range records {
		switch records[i].Type {
		case "CNAME":
			record, err := objMgr.GetCNAMERecord("default", "", records[i].Name)
			if err != nil {
				continue
			}
			_, err = objMgr.DeleteCNAMERecord(record.Ref)
			if err != nil {
				continue
			}
			deleted = append(deleted, libdns.Record{
				Type:  "CNAME",
				Name:  *record.Name,
				Value: *record.Canonical,
			})
		case "TXT":
			record, err := objMgr.GetTXTRecord("default", records[i].Name)
			if err != nil {
				continue
			}
			_, err = objMgr.DeleteTXTRecord(record.Ref)
			if err != nil {
				continue
			}
			deleted = append(deleted, libdns.Record{
				Type:  "TXT",
				Name:  *record.Name,
				Value: *record.Text,
			})
		}
	}

	return deleted, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)

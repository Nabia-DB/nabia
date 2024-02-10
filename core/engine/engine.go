package engine

import (
	"fmt"
	"regexp"
	"sync"
)

type path = string // The key
type ContentType = string
type NabiaRecord struct {
	RawData     []byte
	ContentType ContentType // "Content-Type" https://datatracker.ietf.org/doc/html/rfc2616/#section-14.17
}

func NewNabiaString(s string) *NabiaRecord {
	return &NabiaRecord{RawData: []byte(s), ContentType: "text/plain; charset=UTF-8"}
}

func NewNabiaRecord(data []byte, ct ContentType) *NabiaRecord {
	return &NabiaRecord{RawData: data, ContentType: ct}
}

type NabiaDB struct {
	Records sync.Map
}

func NewNabiaDB() *NabiaDB {
	return &NabiaDB{}
}

// Below are the DB primitives.

// Exists checks if the key name provided exists in the Nabia map. It locks
// to read and unlocks immediately after.
func (ns *NabiaDB) Exists(key string) bool {
	_, ok := ns.Records.Load(key)
	return ok
}

// Read takes a key name and attempts to pull the data from the Nabia DB map.
// Returns a NabiaRecord (if found) and an error (if not found). Callers must
// always check the error returned in the second parameter, as the result cannot
// be used if the "error" field is not nil. This function is safe to call even
// with empty data, because the method applies a mutex.
func (ns *NabiaDB) Read(key string) (NabiaRecord, error) {
	if value, ok := ns.Records.Load(key); ok {
		record := value.(*NabiaRecord)
		return *record, nil
	} else {
		return NabiaRecord{}, fmt.Errorf("key '%s' doesn't exist", key)
	}
}

// Write takes the key and a value of NabiaRecord datatype and places it on the
// database, potentially overwriting whatever was there before, because Write
// has no data safety features preventing the overwriting of data.
func (ns *NabiaDB) Write(key string, value NabiaRecord) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if value.RawData == nil {
		return fmt.Errorf("value cannot be nil")
	}
	if value.ContentType == "" {
		return fmt.Errorf("Content-Type cannot be empty")
	}
	pattern := `^[a-zA-Z0-9]+/[a-zA-Z0-9]+`
	r := regexp.MustCompile(pattern)
	if !r.MatchString(value.ContentType) {
		return fmt.Errorf("Content-Type is not valid")
	} else {
		ns.Records.Store(key, &value)
	}
	return nil
}

// Destroy takes a key and removes it from the map. This method doesn't have
// existence-checking logic. It is safe to use on empty data, it simply doesn't
// do anything if the record doesn't exist.
func (ns *NabiaDB) Destroy(key string) {
	ns.Records.Delete(key)
}

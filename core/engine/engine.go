package engine

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
)

type ContentType = string
type NabiaRecord struct {
	RawData     []byte
	ContentType ContentType // "Content-Type" https://datatracker.ietf.org/doc/html/rfc2616/#section-14.17
}
type stats struct {
	reads  int64
	writes int64
	size   int64
}
type internals struct {
	location string
	stats    stats
}
type NabiaDB struct {
	Records   sync.Map
	internals internals
}

func NewNabiaString(s string) *NabiaRecord {
	return &NabiaRecord{RawData: []byte(s), ContentType: "text/plain; charset=UTF-8"}
}

func NewNabiaRecord(data []byte, ct ContentType) *NabiaRecord {
	return &NabiaRecord{RawData: data, ContentType: ct}
}

// checkOrCreateDB checks if the file exists, and if it doesn't, it creates it.
// The first boolean indicates whether the file already existed, and the second
// boolean indicates whether an error occurred.
func checkOrCreateFile(location string) (bool, error) {
	// Attempt to open the file in read-only mode to check if it exists.
	if _, err := os.Stat(location); err == nil {
		// The file exists.
		return true, nil
	} else if os.IsNotExist(err) {
		// The file does not exist, attempt to create it.
		file, err := os.Create(location)
		if err != nil {
			// Failed to create the file, return the error.
			return false, err
		}
		// Successfully created the file, close it.
		defer file.Close()
		return false, nil
	} else {
		// Some other error occurred when checking the file, return it.
		return false, err
	}
}

func newEmptyDB() *NabiaDB {
	return &NabiaDB{
		Records: sync.Map{},
		internals: internals{
			location: "",
			stats: stats{
				reads:  0, // Initialize reads to 0
				writes: 0, // Initialize writes to 0
				size:   0, // Initialize size to 0, assuming this is the initial size
			},
		},
	}
}

func NewNabiaDB(location string) (*NabiaDB, error) {
	exists, err := checkOrCreateFile(location)
	if err != nil {
		return nil, err
	}
	ndb := newEmptyDB()
	ndb.internals.location = location
	if exists {
		loaded_ndb, err := loadFromFile(location)
		if err != nil {
			return nil, err
		}
		return loaded_ndb, nil
	} else {
		if err := ndb.saveToFile(location); err != nil {
			log.Fatalf("Failed to save to file: %s", err)
		}
	}
	return ndb, nil
}

// Below are the DB primitives.

// Exists checks if the key name provided exists in the Nabia map. It locks
// to read and unlocks immediately after.
func (ns *NabiaDB) Exists(key string) bool {
	if key == "" { // key cannot be empty
		return false
	}
	atomic.AddInt64(&ns.internals.stats.reads, 1)
	_, ok := ns.Records.Load(key)
	return ok
}

// Read takes a key name and attempts to pull the data from the Nabia DB map.
// Returns a NabiaRecord (if found) and an error (if not found). Callers must
// always check the error returned in the second parameter, as the result cannot
// be used if the "error" field is not nil. This function is safe to call even
// with empty data, because the method applies a mutex.
func (ns *NabiaDB) Read(key string) (NabiaRecord, error) {
	if key == "" {
		return NabiaRecord{}, fmt.Errorf("key cannot be empty")
	}
	atomic.AddInt64(&ns.internals.stats.reads, 1)
	if value, ok := ns.Records.Load(key); ok {
		record, ok := value.(NabiaRecord)
		if !ok {
			return NabiaRecord{}, fmt.Errorf("type assertion to NabiaRecord failed")
		}
		return record, nil
	}
	return NabiaRecord{}, fmt.Errorf("key '%s' doesn't exist", key)
}

// Write takes the key and a value of NabiaRecord datatype and places it on the
// database, potentially overwriting whatever was there before, because Write
// has no data safety features preventing the overwriting of data.
func (ns *NabiaDB) Write(key string, value NabiaRecord) error {
	// validation
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
	}
	// writing
	atomic.AddInt64(&ns.internals.stats.writes, 1)
	if !ns.Exists(key) {
		atomic.AddInt64(&ns.internals.stats.size, 1)
	}
	ns.Records.Store(key, value)
	return nil
}

// Destroy takes a key and removes it from the map. This method doesn't have
// existence-checking logic. It is safe to use on empty data, it simply doesn't
// do anything if the record doesn't exist.
func (ns *NabiaDB) Destroy(key string) {
	if ns.Exists(key) {
		atomic.AddInt64(&ns.internals.stats.size, -1)
	}
	ns.Records.Delete(key)
	atomic.AddInt64(&ns.internals.stats.writes, 1)
}

func (ns *NabiaDB) Stop() {
	ns.saveToFile(ns.internals.location)
}

func (ns *NabiaDB) saveToFile(filename string) error {
	// Open or create the file for writing. os.Create truncates the file if it already exists.
	file, err := os.Create(filename)
	if err != nil {
		return err // Return the error if file creation fails
	}
	defer file.Close() // Ensure the file is closed after writing is complete

	// Use a buffered writer for efficient file writing
	writer := bufio.NewWriter(file)
	defer writer.Flush() // Ensure buffered data is flushed to file

	// Create a new gob encoder that writes to the buffered writer
	encoder := gob.NewEncoder(writer)

	// Prepare a regular map to hold the data from sync.Map
	// This is necessary because gob cannot directly encode/decode sync.Map
	data := make(map[string]NabiaRecord)

	// Copy data from sync.Map to the regular map
	ns.Records.Range(func(key, value interface{}) bool {
		strKey, okKey := key.(string)               // Ensure the key is of type string
		nabiaRecord, okValue := value.(NabiaRecord) // Ensure the value is of type NabiaRecord
		if okKey && okValue {
			data[strKey] = nabiaRecord
		}
		return true // Continue iterating over all entries in the sync.Map
	})

	// Encode the regular map into the file
	err = encoder.Encode(data)
	if err != nil {
		return err // Return the error if encoding fails
	}

	return nil // Return nil if the function completes successfully
}

func loadFromFile(filename string) (*NabiaDB, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Use a buffered reader for better performance
	reader := bufio.NewReader(file)
	decoder := gob.NewDecoder(reader)

	// Decode the map
	data := make(map[string]NabiaRecord)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	// Convert the regular map back to a sync.Map
	ndb := newEmptyDB()
	ndb.internals.location = filename
	for key, value := range data {
		ndb.Write(key, value)
		ndb.internals.stats.size++
	}

	return ndb, err
}

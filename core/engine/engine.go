package engine

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type NabiaRecord struct {
	RawData []byte
}

type dataActivity struct {
	reads  int64
	writes int64
	size   int64
}
type timestamps struct {
	lastSave  time.Time
	lastLoad  time.Time
	lastRead  time.Time
	lastWrite time.Time
}
type metrics struct {
	dataActivity dataActivity
	timestamps   timestamps
}
type internals struct {
	location string
	metrics  metrics
}
type NabiaDB struct {
	records   sync.Map
	internals internals
}

func NewNabiaRecord(data []byte) (*NabiaRecord, error) { // TODO this function can be expanded later
	return &NabiaRecord{RawData: data}, nil
}

func (nr *NabiaRecord) GetRawData() interface{} {
	return nr.RawData
}

// checkOrCreateDB checks if the file exists, and if it doesn't, it creates it.
// The first boolean indicates whether the file already existed, and the second
// boolean indicates whether an error occurred.
func checkOrCreateFile(location string) (bool, error) {
	if location == "" {
		return false, fmt.Errorf("location cannot be empty")
	}
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
		records: sync.Map{},
		internals: internals{
			location: "",
			metrics: metrics{
				dataActivity: dataActivity{
					reads:  0,
					writes: 0,
					size:   0,
				},
				timestamps: timestamps{
					lastSave:  time.Now(),
					lastLoad:  time.Now(),
					lastRead:  time.Now(),
					lastWrite: time.Now(),
				},
			},
		},
	}
}

func NewNabiaDB(location string) (*NabiaDB, error) {
	ndb := newEmptyDB()
	ndb.internals.location = location
	if err := ndb.saveToFile(location); err != nil {
		return nil, err
	}
	return ndb, nil
}

// TODO NabiaDBFromFile must be reimplemented
//func NabiaDBFromFile(location string) (*NabiaDB, error) {
//	return loadFromFile(location)
//}

// Below are the DB primitives.

// Exists checks if the key name provided exists in the Nabia map. It locks
// to read and unlocks immediately after.
// +1 read
func (ns *NabiaDB) Exists(key string) bool {
	if key == "" { // key cannot be empty
		return false
	}
	ns.internals.metrics.timestamps.lastRead = time.Now()
	atomic.AddInt64(&ns.internals.metrics.dataActivity.reads, 1)
	_, ok := ns.records.Load(key)
	return ok
}

// Read takes a key name and attempts to pull the data from the Nabia DB map.
// Returns a NabiaRecord if found and an error if not found. Callers must
// always check the error returned in the second parameter, as the result cannot
// be used if the "error" field is not nil. This function is safe to call even
// with empty data, because the method applies a mutex.
// +1 read
func (ns *NabiaDB) Read(key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	ns.internals.metrics.timestamps.lastRead = time.Now()
	atomic.AddInt64(&ns.internals.metrics.dataActivity.reads, 1)
	if value, ok := ns.records.Load(key); ok {
		if record, ok := value.([]byte); ok {
			return record, nil
		}
		panic("value is not a []byte; this should never happen, something has gone very wrong")
	}
	return nil, fmt.Errorf("key %q doesn't exist", key)
}

// Write takes the key and a value of NabiaRecord datatype and places it on the
// database, potentially overwriting whatever was there before, because Write
// has no data safety features preventing the overwriting of data.
// +1 write when validation passes
// +1 size if the key is new
func (ns *NabiaDB) Write(key string, value []byte) error {
	// validation
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if bytes.Equal(value, []byte{}) {
		return fmt.Errorf("value cannot be nil")
	}
	// writing
	ns.internals.metrics.timestamps.lastWrite = time.Now()
	atomic.AddInt64(&ns.internals.metrics.dataActivity.writes, 1)
	if !ns.Exists(key) {
		atomic.AddInt64(&ns.internals.metrics.dataActivity.size, 1)
	}
	ns.records.Store(key, value)
	return nil
}

// Delete takes a key and removes it from the map. This method doesn't have
// existence-checking logic. It is safe to use on empty data, it simply doesn't
// do anything if the record doesn't exist.
// -1 size if the key exists
// +1 write
func (ns *NabiaDB) Delete(key string) {
	if ns.Exists(key) {
		atomic.AddInt64(&ns.internals.metrics.dataActivity.size, -1)
	}
	ns.records.Delete(key)
	ns.internals.metrics.timestamps.lastWrite = time.Now()
	atomic.AddInt64(&ns.internals.metrics.dataActivity.writes, 1)
}

func (ns *NabiaDB) Stop() {
	ns.saveToFile(ns.internals.location)
	// TODO emit a shutdown signal
}

// TODO: Saving and loading must be reimplemented
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
	data := make(map[string]NabiaRecord) // The string is always the key regardless of data type

	// Copy data from sync.Map to the regular map
	ns.records.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.([]byte); ok {
				data[k] = NabiaRecord{RawData: v}
			}
		}
		return true
	})

	// Encode the regular map into the file
	err = encoder.Encode(data)
	if err != nil {
		return err // Return the error if encoding fails
	}

	ns.internals.metrics.timestamps.lastSave = time.Now()
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
		ndb.Write(fmt.Sprintf("%v", key), value.GetRawData().([]byte))
		ndb.internals.metrics.dataActivity.size++
	}

	ndb.internals.metrics.timestamps.lastLoad = time.Now()

	return ndb, nil
}

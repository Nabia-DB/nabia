package engine

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNabiaDB_SaveLoadCycle(t *testing.T) {
	// Setup
	dbLocation := "testNabiaDB.db"
	defer os.Remove(dbLocation) // Clean up after the test

	db, err := NewNabiaDB(dbLocation)
	if err != nil {
		t.Fatalf("Failed to create NabiaDB: %v", err)
	}

	// Generate and store random records
	keys := []string{"key1", "key2"}
	expectedRecords := make(map[string]NabiaRecord)
	for _, key := range keys {
		val := RandStringBytes(10)
		record := NewNabiaString(val)
		if err := db.Write(key, *record); err != nil {
			t.Fatalf("Failed to write record for key %s: %v", key, err)
		}
		expectedRecords[key] = *record
	}

	// Save to file
	if err := db.saveToFile(dbLocation); err != nil {
		t.Fatalf("Failed to save database to file: %v", err)
	}

	// Attempt to reload from file
	reloadedDB, err := loadFromFile(dbLocation)
	if err != nil {
		t.Fatalf("Failed to load database from file: %v", err)
	}

	// Verify that the reloaded data matches the expected data
	for key, expectedRecord := range expectedRecords {
		actualRecord, err := reloadedDB.Read(key)
		if err != nil {
			t.Errorf("Failed to read key %s from reloaded database: %v", key, err)
		}

		if !reflect.DeepEqual(actualRecord, expectedRecord) {
			t.Errorf("Mismatch for key %s: got %+v, want %+v", key, actualRecord, expectedRecord)
		}
	}
}

// RandStringBytes generates a random string of n bytes.
func RandStringBytes(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func TestFileSavingAndLoading(t *testing.T) {
	location := "test.db"
	exists, err := checkOrCreateFile(location)
	if err != nil {
		t.Fatalf("failed to check or create file: %s", err) // Unknown error
	} else {
		if exists {
			if err := os.Remove(location); err != nil {
				t.Fatalf("failed to remove test.db: %s", err) // Unknown error
			}
		}
	}
	defer os.Remove(location)
	nabia_db, err := NewNabiaDB(location)
	if err != nil {
		t.Fatalf("failed to create NabiaDB: %s", err) // Unknown error
	}
	if err := nabia_db.Write("A", *NewNabiaString("Value_A")); err != nil { // Failure when writing a value
		t.Errorf("failed to write to NabiaDB: %s", err) // Unknown error
	}
	if err := nabia_db.saveToFile(location); err != nil {
		t.Fatalf("failed to save NabiaDB to file: %s", err) // Unknown error
	}
	nabia_db, err = loadFromFile(location)
	if err != nil {
		t.Fatalf("failed to load NabiaDB from file: %s", err) // Unknown error
	}
	exists, err = checkOrCreateFile(location)
	if err != nil {
		t.Fatalf("failed to check or create file: %s", err)
	} else {
		if !exists {
			t.Errorf("file should exist: %s", err)
		}
	}
	if err := os.Remove(location); err != nil { // Deleting DB from disk
		t.Fatalf("failed to remove test.db: %s", err)
	}
	_, err = loadFromFile(location)
	if !strings.Contains(err.Error(), "no such file or directory") { // Attempting to read a file that doesn't exist should never succeed
		t.Errorf("should not succeed when attempting to load a non-existant file: %s", err)
	}
	if err := nabia_db.saveToFile(location); err != nil { // Attempting to save after deletion
		t.Fatalf("failed to save NabiaDB to file: %s", err)
	}
	nabia_db, err = loadFromFile(location) // Attempting to load the database once again
	if err != nil {
		t.Fatalf("failed to load NabiaDB from file: %s", err) // Unknown error
	}
	nr, err := nabia_db.Read("A") // Attempting to read the value saved earlier
	if err != nil {
		t.Fatalf("failed to read from NabiaDB: %s", err) // Unknown error
	} else {
		expectedData := []byte("Value_A")
		expectedContentType := "text/plain; charset=UTF-8"
		if !bytes.Equal(nr.RawData, expectedData) || nr.ContentType != expectedContentType {
			t.Errorf("failed to read the correct value from NabiaDB: %s", err)
		}
	}
	nr, err = nabia_db.Read("B")
	if err == nil {
		t.Error("should not succeed when attempting to read a non-existent key")
	}
	if err := os.Remove(location); err != nil { // Final DB deletion
		t.Fatalf("failed to remove test.db: %s", err)
	}
}

func TestCRUD(t *testing.T) { // Create, Read, Update, Destroy

	var nabia_read NabiaRecord
	var expected []byte
	var expectedContentType ContentType

	nabia_db, err := NewNabiaDB("nabia.db")
	if err != nil {
		t.Errorf("Failed to create NabiaDB: %s", err)
	}

	if nabia_db.Exists("A") {
		t.Error("Uninitialised database contains elements!")
	}
	//CREATE
	s := NewNabiaString("Value_A")
	nabia_db.Write("A", *s)
	if !nabia_db.Exists("A") {
		t.Error("Database is not writing items correctly!")
	}
	//READ
	nabia_read, err = nabia_db.Read("A")
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	expected = []byte("Value_A")
	expectedContentType = "text/plain; charset=UTF-8"
	for i, e := range nabia_read.RawData {
		if e != expected[i] || nabia_read.ContentType != expectedContentType {
			t.Errorf("\"Read\" returns unexpected data or ContentType!\nGot %q, expected %q", nabia_read, expected)
		}
	}
	//UPDATE
	s1 := NewNabiaRecord([]byte("Modified value"), "application/json; charset=UTF-8")
	nabia_db.Write("A", *s1)
	if !nabia_db.Exists("A") {
		t.Errorf("Overwritten item doesn't exist!")
	}
	nabia_read, err = nabia_db.Read("A")
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	expected = []byte("Modified value")
	expectedContentType = "application/json; charset=UTF-8"
	for i, e := range nabia_read.RawData {
		if e != expected[i] || nabia_read.ContentType != expectedContentType {
			t.Errorf("\"Write\" on an existing item saves unexpected data or ContentType!\nGot %q, expected %q", nabia_read, expected)
		}
	}
	//DESTROY
	if !nabia_db.Exists("A") {
		t.Error("Can't destroy item because it doesn't exist!")
	}
	nabia_db.Destroy("A")
	if nabia_db.Exists("A") {
		t.Error("\"Destroy\" isn't working!\nDeleted item still exists in DB.")
	}

	// Test for unknown ContentType
	s2 := NewNabiaRecord([]byte("Unknown ContentType Value"), "unknown/type; charset=unknown")
	nabia_db.Write("B", *s2)
	nabia_read, _ = nabia_db.Read("B")
	if nabia_read.ContentType != "unknown/type; charset=unknown" {
		t.Error("Content type not saving correctly")
	}

	// Test for incorrect ContentType
	s3 := NewNabiaRecord([]byte("Incorrect ContentType Value"), "QWERTYABCD")
	incorrect_content_type := nabia_db.Write("C", *s3)
	if !strings.Contains(incorrect_content_type.Error(), "Content-Type is not valid") {
		t.Error("malformed Content-Type should not be allowed")
	}
	nabia_read, err = nabia_db.Read("C")
	if err == nil {
		t.Error("malformed Content-Type should not be written to the database")
	}

	// Test for non-existent item
	nabia_db.Destroy("C")
	if nabia_db.Exists("C") {
		t.Error("\"Destroy\" isn't working!\nNon-existent item appears to exist in DB.")
	}

	// Test for incorrect key
	incorrect_key := nabia_db.Write("", *s) // This should not be allowed
	if !strings.Contains(incorrect_key.Error(), "key cannot be empty") {
		t.Error("Empty key should not be allowed")
	}

	// Test for incorrect values
	incorrect_value1 := nabia_db.Write("/A", NabiaRecord{}) // This should not be allowed
	if !strings.Contains(incorrect_value1.Error(), "value cannot be nil") {
		t.Error("Empty NabiaRecord should not be allowed")
	}
	incorrect_value2 := nabia_db.Write("/A", NabiaRecord{nil, "application/json; charset=UTF-8"}) // This should not be allowed
	if !strings.Contains(incorrect_value2.Error(), "value cannot be nil") {
		t.Error("nil NabiaRecord RawData should not be allowed")
	}
	incorrect_value3 := nabia_db.Write("/A", NabiaRecord{[]byte("Value_A"), ""}) // This should not be allowed
	if !strings.Contains(incorrect_value3.Error(), "Content-Type cannot be empty") {
		t.Error("Empty NabiaRecord ContentType should not be allowed")
	}

	// Concurrency test with Destroy operation
	var wg sync.WaitGroup
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("Key_%d", i)
			value := NewNabiaRecord([]byte(fmt.Sprintf("Value_%d", i)), "text/plain; charset=UTF-8")
			operation := rand.Intn(3)
			switch operation {
			case 0:
				// Destroy before writing
				nabia_db.Destroy(key)
				if nabia_db.Exists(key) {
					t.Errorf("Destroy operation failed before writing for key: %s", key)
				}
				nabia_db.Write(key, *value)
			case 1:
				// Destroy after writing and verifying the value
				nabia_db.Write(key, *value)
				readValue, err := nabia_db.Read(key)
				if err != nil || !bytes.Equal(readValue.RawData, value.RawData) || readValue.ContentType != value.ContentType {
					t.Errorf("Write or Read operation failed for key: %s", key)
				}
				nabia_db.Destroy(key)
				if nabia_db.Exists(key) {
					t.Errorf("Destroy operation failed after writing for key: %s", key)
				}
			case 2:
				// Overwrite and check value again after checking value with first write
				nabia_db.Write(key, *value)
				readValue, err := nabia_db.Read(key)
				if err != nil || !bytes.Equal(readValue.RawData, value.RawData) || readValue.ContentType != value.ContentType {
					t.Errorf("First Write or Read operation failed for key: %s", key)
				}
				value2 := NewNabiaRecord([]byte(fmt.Sprintf("New_Value_%d", i)), "application/json; charset=UTF-8")
				nabia_db.Write(key, *value2)
				readValue2, err := nabia_db.Read(key)
				if err != nil || !bytes.Equal(readValue2.RawData, value2.RawData) || readValue2.ContentType != value2.ContentType {
					t.Errorf("Second Write or Read operation failed for key: %s", key)
				}
			}
		}(i)
	}
	wg.Wait()

}

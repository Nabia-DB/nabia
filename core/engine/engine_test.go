package engine

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

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

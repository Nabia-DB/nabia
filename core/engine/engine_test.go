package engine

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func TestCRUD(t *testing.T) { // Create, Read, Update, Destroy
	nabia_db := NewNabiaDB()

	var nabia_read NabiaRecord
	var err error
	var expected []byte
	var expectedContentType ContentType

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
	nabia_read, err = nabia_db.Read("B")
	if nabia_read.ContentType != "unknown/type; charset=unknown" {
		t.Errorf("Content type not saving correctly")
	}

	// Test for non-existent item
	nabia_db.Destroy("C")
	if nabia_db.Exists("C") {
		t.Error("\"Destroy\" isn't working!\nNon-existent item appears to exist in DB.")
	}

	// Concurrency test with Destroy operation
	var wg sync.WaitGroup
	for i := 0; i < 100000; i++ {
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

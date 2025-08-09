package engine

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileSavingAndLoading(t *testing.T) {
	location := "filesaving.db"
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
	os.Remove(location)
	nabiaDB, err := NewNabiaDB(location)
	if err != nil {
		t.Fatalf("failed to create NabiaDB: %s", err) // Unknown error
	}
	defer os.Remove(location)
	if err := nabiaDB.Write("A", []byte("Value_A")); err != nil { // Failure when writing a value
		t.Errorf("failed to write to NabiaDB: %s", err) // Unknown error
	}
	if err := nabiaDB.SaveToFile(location); err != nil {
		t.Fatalf("failed to save NabiaDB to file: %s", err) // Unknown error
	}
	nabiaDB, err = LoadFromFile(location)
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
	_, err = LoadFromFile(location)
	if !strings.Contains(err.Error(), "no such file or directory") { // Attempting to read a file that doesn't exist should never succeed
		t.Errorf("should not succeed when attempting to load a non-existant file: %s", err)
	}
	if err := nabiaDB.SaveToFile(location); err != nil { // Attempting to save after deletion
		t.Fatalf("failed to save NabiaDB to file: %s", err)
	}
	nabiaDB_loaded, err := LoadFromFile(location) // Attempting to load the database once again
	if err != nil {
		t.Fatalf("failed to load NabiaDB from file: %s", err) // Unknown error
	}
	nr, err := nabiaDB_loaded.Read("A") // Attempting to read the value saved earlier
	if err != nil {
		t.Fatalf("failed to read from NabiaDB: %s", err) // Unknown error
	} else {
		expectedData := []byte("Value_A")
		if !bytes.Equal(nr, expectedData) {
			t.Errorf("failed to read the correct value from NabiaDB: %s", err)
		}
	}
	_, err = nabiaDB_loaded.Read("B")
	if err == nil {
		t.Error("should not succeed when attempting to read a non-existent key")
	}
	if err := os.Remove(location); err != nil { // Final DB deletion
		t.Fatalf("failed to remove test.db: %s", err)
	}
}

func TestCRUD(t *testing.T) { // Create, Read, Update, Destroy

	var nabia_read []byte
	var expected []byte
	expected_stats := dataActivity{reads: 0, writes: 0, size: 0}

	nabiaDB, err := NewNabiaDB("crud.db")
	if err != nil {
		t.Errorf("Failed to create NabiaDB: %s", err)
	}
	defer os.Remove("crud.db")

	if nabiaDB.Exists("A") {
		t.Error("Uninitialised database contains elements!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	//CREATE
	nabiaDB.Write("A", []byte("Value_A"))
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	atomic.AddInt64(&expected_stats.size, 1)
	if !nabiaDB.Exists("A") {
		t.Error("Database is not writing items correctly!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	//READ
	nabia_read, err = nabiaDB.Read("A")
	atomic.AddInt64(&expected_stats.reads, 1)
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	expected = []byte("Value_A")
	if !bytes.Equal(expected, nabia_read) {
		t.Errorf("\"Read\" returns unexpected data!\nGot %q, expected %q", nabia_read, expected)
	}
	//UPDATE
	s1 := []byte("Modified value")
	if err != nil {
		t.Errorf(("Failed to create NabiaRecord: %s"), err)
	}
	nabiaDB.Write("A", s1)
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	if !nabiaDB.Exists("A") {
		t.Errorf("Overwritten item doesn't exist!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	nabia_read, err = nabiaDB.Read("A")
	if err != nil {
		t.Errorf("\"Read\" returns an unexpected error:\n%q", err.Error())
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	expected = []byte("Modified value")
	bytes.Equal(expected, nabia_read)
	//DESTROY
	if !nabiaDB.Exists("A") {
		t.Error("Can't destroy item because it doesn't exist!")
	}
	atomic.AddInt64(&expected_stats.reads, 1)
	nabiaDB.Delete("A")
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	atomic.AddInt64(&expected_stats.size, -1)
	if nabiaDB.Exists("A") {
		t.Error("\"Destroy\" isn't working!\nDeleted item still exists in DB.")
	}
	atomic.AddInt64(&expected_stats.reads, 1)

	// Test for non-existent item
	nabiaDB.Delete("C") // This should never fail regardless of whether the key exists
	atomic.AddInt64(&expected_stats.reads, 1)
	atomic.AddInt64(&expected_stats.writes, 1)
	if nabiaDB.Exists("C") {
		t.Error("\"Destroy\" isn't working!\nNon-existent item appears to exist in DB.")
	}
	atomic.AddInt64(&expected_stats.reads, 1)

	// Test for incorrect key
	incorrect_key := nabiaDB.Write("", []byte("This should fail")) // This should not be allowed
	if !strings.Contains(incorrect_key.Error(), "key cannot be empty") {
		t.Error("Empty key should not be allowed")
	}

	// Test for incorrect values
	incorrect_value := nabiaDB.Write("/A", []byte{}) // This should not be allowed
	if incorrect_value == nil || !strings.Contains(incorrect_value.Error(), "value cannot be nil") {
		t.Error("Empty NabiaRecord should not be allowed")
	}

	// Test the metrics
	if !reflect.DeepEqual(nabiaDB.internals.metrics.dataActivity, expected_stats) {
		t.Errorf("Stats are not as expected.\nExpected: %+v\nGot: %+v", expected_stats, nabiaDB.internals.metrics.dataActivity)
	}

}

func TestNewNabiaRecord(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "Valid data",
			data:        []byte("Hello, World!"),
			expectError: false,
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: false,
		},
		{
			name:        "Binary data",
			data:        []byte{0x00, 0x01, 0x02, 0xff},
			expectError: false,
		},
		{
			name:        "Nil data",
			data:        nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := NewNabiaRecord(tt.data)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError {
				if !bytes.Equal(record.RawData, tt.data) {
					t.Errorf("Expected data %v, got %v", tt.data, record.RawData)
				}
			}
		})
	}
}

func TestCheckOrCreateFile(t *testing.T) {
	t.Run("Create new file", func(t *testing.T) {
		testFile := "test_create.db"
		defer os.Remove(testFile)
		
		exists, err := checkOrCreateFile(testFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if exists {
			t.Error("File should not exist initially")
		}
		
		// Verify file was created
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("File should have been created")
		}
	})

	t.Run("File already exists", func(t *testing.T) {
		testFile := "test_exists.db"
		
		// Create the file first
		file, err := os.Create(testFile)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		file.Close()
		defer os.Remove(testFile)
		
		exists, err := checkOrCreateFile(testFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !exists {
			t.Error("File should exist")
		}
	})

	t.Run("Empty location", func(t *testing.T) {
		_, err := checkOrCreateFile("")
		if err == nil {
			t.Error("Expected error for empty location")
		}
		if !strings.Contains(err.Error(), "location cannot be empty") {
			t.Errorf("Expected 'location cannot be empty' error, got: %v", err)
		}
	})

	t.Run("Invalid path", func(t *testing.T) {
		// Try to create a file in a directory that doesn't exist
		invalidPath := "/nonexistent/directory/file.db"
		_, err := checkOrCreateFile(invalidPath)
		if err == nil {
			t.Error("Expected error for invalid path")
		}
	})
}

func TestNewEmptyDB(t *testing.T) {
	db := newEmptyDB()
	
	if db == nil {
		t.Fatal("newEmptyDB() returned nil")
	}
	
	if db.internals.location != "" {
		t.Errorf("Expected empty location, got: %s", db.internals.location)
	}
	
	if db.internals.metrics.dataActivity.reads != 0 ||
		db.internals.metrics.dataActivity.writes != 0 ||
		db.internals.metrics.dataActivity.size != 0 {
		t.Error("Expected zero metrics for new empty DB")
	}
}

func TestNewNabiaDB(t *testing.T) {
	t.Run("Valid location", func(t *testing.T) {
		testFile := "test_new.db"
		defer os.Remove(testFile)
		
		db, err := NewNabiaDB(testFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		if db.internals.location != testFile {
			t.Errorf("Expected location %s, got %s", testFile, db.internals.location)
		}
		
		// Verify file was created
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Database file should have been created")
		}
	})

	t.Run("Invalid location", func(t *testing.T) {
		_, err := NewNabiaDB("/nonexistent/directory/test.db")
		if err == nil {
			t.Error("Expected error for invalid location")
		}
	})
}

func TestReadEmptyKey(t *testing.T) {
	db := newEmptyDB()
	
	_, err := db.Read("")
	if err == nil {
		t.Error("Expected error for empty key")
	}
	if !strings.Contains(err.Error(), "key cannot be empty") {
		t.Errorf("Expected 'key cannot be empty' error, got: %v", err)
	}
}

func TestReadNonExistentKey(t *testing.T) {
	db := newEmptyDB()
	
	_, err := db.Read("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
	if !strings.Contains(err.Error(), "doesn't exist") {
		t.Errorf("Expected 'doesn't exist' error, got: %v", err)
	}
}

func TestWriteValidation(t *testing.T) {
	db := newEmptyDB()
	
	tests := []struct {
		name        string
		key         string
		value       []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Empty key",
			key:         "",
			value:       []byte("value"),
			expectError: true,
			errorMsg:    "key cannot be empty",
		},
		{
			name:        "Empty value",
			key:         "key",
			value:       []byte{},
			expectError: true,
			errorMsg:    "value cannot be nil",
		},
		{
			name:        "Valid key and value",
			key:         "valid-key",
			value:       []byte("valid-value"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.Write(tt.key, tt.value)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Expected error message containing '%s', got: %v", tt.errorMsg, err)
			}
		})
	}
}

func TestExistsEmptyKey(t *testing.T) {
	db := newEmptyDB()
	
	exists := db.Exists("")
	if exists {
		t.Error("Empty key should not exist")
	}
}

func TestMetricsTracking(t *testing.T) {
	db := newEmptyDB()
	
	// Initial metrics should be zero
	if db.internals.metrics.dataActivity.reads != 0 {
		t.Error("Initial read count should be zero")
	}
	if db.internals.metrics.dataActivity.writes != 0 {
		t.Error("Initial write count should be zero")
	}
	if db.internals.metrics.dataActivity.size != 0 {
		t.Error("Initial size should be zero")
	}
	
	// Test read metrics
	db.Exists("test-key") // Should increment reads
	if db.internals.metrics.dataActivity.reads != 1 {
		t.Errorf("Expected 1 read, got %d", db.internals.metrics.dataActivity.reads)
	}
	
	// Test write metrics
	db.Write("test-key", []byte("test-value"))
	if db.internals.metrics.dataActivity.writes != 1 {
		t.Errorf("Expected 1 write, got %d", db.internals.metrics.dataActivity.writes)
	}
	if db.internals.metrics.dataActivity.size != 1 {
		t.Errorf("Expected size 1, got %d", db.internals.metrics.dataActivity.size)
	}
	
	// Test overwrite (should increment writes but not size)
	db.Write("test-key", []byte("new-value"))
	if db.internals.metrics.dataActivity.writes != 2 {
		t.Errorf("Expected 2 writes, got %d", db.internals.metrics.dataActivity.writes)
	}
	if db.internals.metrics.dataActivity.size != 1 {
		t.Errorf("Expected size still 1, got %d", db.internals.metrics.dataActivity.size)
	}
	
	// Test delete
	db.Delete("test-key")
	if db.internals.metrics.dataActivity.writes != 3 {
		t.Errorf("Expected 3 writes, got %d", db.internals.metrics.dataActivity.writes)
	}
	if db.internals.metrics.dataActivity.size != 0 {
		t.Errorf("Expected size 0, got %d", db.internals.metrics.dataActivity.size)
	}
}

func TestDeleteNonExistentKey(t *testing.T) {
	db := newEmptyDB()
	
	initialWrites := db.internals.metrics.dataActivity.writes
	initialSize := db.internals.metrics.dataActivity.size
	
	db.Delete("nonexistent-key")
	
	// Should still increment writes but not change size
	if db.internals.metrics.dataActivity.writes != initialWrites+1 {
		t.Errorf("Expected writes to increment, got %d", db.internals.metrics.dataActivity.writes)
	}
	if db.internals.metrics.dataActivity.size != initialSize {
		t.Errorf("Expected size to remain same, got %d", db.internals.metrics.dataActivity.size)
	}
}

func TestLargeKeys(t *testing.T) {
	db := newEmptyDB()
	
	// Test with very long key
	longKey := strings.Repeat("a", 10000)
	err := db.Write(longKey, []byte("value"))
	if err != nil {
		t.Errorf("Unexpected error with long key: %v", err)
	}
	
	// Verify we can read it back
	value, err := db.Read(longKey)
	if err != nil {
		t.Errorf("Unexpected error reading long key: %v", err)
	}
	if string(value) != "value" {
		t.Errorf("Expected 'value', got '%s'", string(value))
	}
}

func TestLargeValues(t *testing.T) {
	db := newEmptyDB()
	
	// Test with large value (1MB)
	largeValue := bytes.Repeat([]byte("x"), 1024*1024)
	err := db.Write("large-key", largeValue)
	if err != nil {
		t.Errorf("Unexpected error with large value: %v", err)
	}
	
	// Verify we can read it back
	value, err := db.Read("large-key")
	if err != nil {
		t.Errorf("Unexpected error reading large value: %v", err)
	}
	if !bytes.Equal(value, largeValue) {
		t.Error("Large value not preserved correctly")
	}
}

func TestSpecialCharactersInKeys(t *testing.T) {
	db := newEmptyDB()
	
	specialKeys := []string{
		"key with spaces",
		"key/with/slashes",
		"key-with-dashes",
		"key_with_underscores",
		"key.with.dots",
		"key@with@symbols",
		"キー", // Unicode key
		"🔑",   // Emoji key
	}
	
	for _, key := range specialKeys {
		t.Run(fmt.Sprintf("Key: %s", key), func(t *testing.T) {
			value := []byte(fmt.Sprintf("value for %s", key))
			
			err := db.Write(key, value)
			if err != nil {
				t.Errorf("Unexpected error writing key '%s': %v", key, err)
			}
			
			readValue, err := db.Read(key)
			if err != nil {
				t.Errorf("Unexpected error reading key '%s': %v", key, err)
			}
			
			if !bytes.Equal(readValue, value) {
				t.Errorf("Value mismatch for key '%s'", key)
			}
		})
	}
}

func TestTimestamps(t *testing.T) {
	db := newEmptyDB()
	
	initialTime := db.internals.metrics.timestamps.lastRead
	
	// Wait a bit to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)
	
	db.Exists("test-key")
	
	if !db.internals.metrics.timestamps.lastRead.After(initialTime) {
		t.Error("lastRead timestamp should have been updated")
	}
	
	initialWrite := db.internals.metrics.timestamps.lastWrite
	time.Sleep(1 * time.Millisecond)
	
	db.Write("test-key", []byte("value"))
	
	if !db.internals.metrics.timestamps.lastWrite.After(initialWrite) {
		t.Error("lastWrite timestamp should have been updated")
	}
}

func TestSaveToFileErrors(t *testing.T) {
	db := newEmptyDB()
	
	// Try to save to an invalid path
	err := db.SaveToFile("/nonexistent/directory/file.db")
	if err == nil {
		t.Error("Expected error saving to invalid path")
	}
}

func TestLoadFromFileErrors(t *testing.T) {
	// Try to load from non-existent file
	_, err := LoadFromFile("nonexistent.db")
	if err == nil {
		t.Error("Expected error loading non-existent file")
	}
	
	// Create a file with invalid gob data
	invalidFile := "invalid.db"
	err = os.WriteFile(invalidFile, []byte("invalid gob data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}
	defer os.Remove(invalidFile)
	
	_, err = LoadFromFile(invalidFile)
	if err == nil {
		t.Error("Expected error loading invalid gob file")
	}
}

// Benchmark tests for performance
func BenchmarkWrite(b *testing.B) {
	db := newEmptyDB()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		db.Write(key, value)
	}
}

func BenchmarkRead(b *testing.B) {
	db := newEmptyDB()
	
	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		db.Write(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%1000)
		db.Read(key)
	}
}

func BenchmarkExists(b *testing.B) {
	db := newEmptyDB()
	
	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		db.Write(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%1000)
		db.Exists(key)
	}
}

func BenchmarkDelete(b *testing.B) {
	db := newEmptyDB()
	
	// Pre-populate with data
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		db.Write(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		db.Delete(key)
	}
}

func TestConcurrency(t *testing.T) {
	expected_stats := dataActivity{reads: 0, writes: 0, size: 0}
	nabiaDB, err := NewNabiaDB("concurrency.db")
	if err != nil {
		t.Errorf("Failed to create NabiaDB: %s", err)
	}
	defer os.Remove("concurrency.db")
	// Concurrency test with Destroy operation
	var wg sync.WaitGroup
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("Key_%d", i)
			value := []byte(fmt.Sprintf("Value_%d", i))
			operation := rand.Intn(3)
			switch operation {
			case 0:
				// Destroy before writing
				nabiaDB.Delete(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				if nabiaDB.Exists(key) {
					t.Errorf("Destroy operation failed before writing for key: %s", key)
				}
				atomic.AddInt64(&expected_stats.reads, 1)
				nabiaDB.Write(key, value)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.size, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
			case 1:
				// Delete after writing and verifying the value
				nabiaDB.Write(key, value)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				atomic.AddInt64(&expected_stats.size, 1)
				readValue, err := nabiaDB.Read(key)
				if err != nil || !bytes.Equal(readValue, value) {
					t.Errorf("Write or Read operation failed for key: %s", key)
				}
				atomic.AddInt64(&expected_stats.reads, 1)
				nabiaDB.Delete(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				atomic.AddInt64(&expected_stats.size, -1)
				if nabiaDB.Exists(key) {
					t.Errorf("Delete operation failed after writing for key: %s", key)
				}
				atomic.AddInt64(&expected_stats.reads, 1)
			case 2:
				// Overwrite and check value again after checking value with first write
				nabiaDB.Write(key, value) // first write
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				atomic.AddInt64(&expected_stats.size, 1)
				readValue, err := nabiaDB.Read(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				if err != nil || !bytes.Equal(readValue, value) {
					t.Errorf("First Write or Read operation failed for key: %s", key)
				}
				value2 := []byte(fmt.Sprintf("New_Value_%d", i))
				nabiaDB.Write(key, value2) // overwrite
				atomic.AddInt64(&expected_stats.reads, 1)
				atomic.AddInt64(&expected_stats.writes, 1)
				readValue2, err := nabiaDB.Read(key)
				atomic.AddInt64(&expected_stats.reads, 1)
				if err != nil || !bytes.Equal(readValue2, value2) {
					t.Errorf("Second Write or Read operation failed for key: %s", key)
				}
			}
		}(i)
	}
	wg.Wait()
	if !reflect.DeepEqual(nabiaDB.internals.metrics.dataActivity, expected_stats) {
		t.Errorf("Stats are not as expected.\nExpected: %+v\nGot: %+v", expected_stats, nabiaDB.internals.metrics.dataActivity)
	}

}

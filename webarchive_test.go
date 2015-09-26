package webarchive

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardlehane/siegfried/pkg/core/siegreader"
)

func ExampleBlackbook() {
	f, _ := os.Open("examples/IAH-20080430204825-00000-blackbook.arc")
	// NewReader(io.Reader) can be used to read WARC, ARC or gzipped WARC or ARC files
	rdr, err := NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	// use Next() to iterate through all records in the WARC or ARC file
	for record, err := rdr.Next(); err == nil; record, err = rdr.Next() {
		// records implement the io.Reader interface
		i, err := io.Copy(ioutil.Discard, record)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Read: %d bytes\n", i)
		// records also have URL(), Date() and Size() methods
		fmt.Printf("URL: %s, Date: %v, Size: %d\n", record.URL(), record.Date(), record.Size())
		// the Fields() method returns all the fields in the WARC or ARC record
		for key, values := range record.Fields() {
			fmt.Printf("Field key: %s, Field values: %v\n", key, values)
		}
	}
	f, _ = os.Open("examplesIAH-20080430204825-00000-blackbook.warc.gz")
	defer f.Close()
	// readers can Reset() to reuse the underlying buffers
	err = rdr.Reset(f)
	// the Close() method should be used if you pass in gzipped files, it is a nop for non-gzipped files
	defer rdr.Close()
	// NextPayload() skips non-resource, conversion or response records and merges continuations into single records.
	// It also strips HTTP headers from response records. After stripping, those HTTP headers are available alongside
	// the WARC headers in the record.Fields() map.
	for record, err := rdr.NextPayload(); err == nil; record, err = rdr.NextPayload() {
		i, err := io.Copy(ioutil.Discard, record)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Read: %d bytes\n", i)
		// any skipped HTTP headers can be retrieved from the Fields() map
		for key, values := range record.Fields() {
			fmt.Printf("Field key: %s, Field values: %v\n", key, values)
		}
	}
}

func opener(t *testing.T) func(string) (Reader, Reader) {
	var wrdr, wrdr2 Reader
	buffers := siegreader.New()
	return func(path string) (Reader, Reader) {
		buf, _ := ioutil.ReadFile(path)
		rdr := bytes.NewReader(buf)
		rdr2 := bytes.NewReader(buf)
		sbuf, _ := buffers.Get(rdr)
		var err error
		if wrdr == nil {
			wrdr, err = NewReader(siegreader.ReaderFrom(sbuf))
		} else {
			err = wrdr.Reset(siegreader.ReaderFrom(sbuf))
		}
		if err != nil {
			if strings.Index(path, "invalid") != -1 {
				return nil, nil
			}
			t.Fatalf("test case: %s; error: %v", path, err)
		}
		if wrdr2 == nil {
			wrdr2, err = NewReader(rdr2)
		} else {
			err = wrdr2.Reset(rdr2)
		}
		if err != nil {
			if strings.Index(path, "invalid") != -1 {
				return nil, nil
			}
			t.Fatalf("test case: %s; error: %v", path, err)
		}
		return wrdr, wrdr2
	}
}

func TestReaders(t *testing.T) {
	open := opener(t)
	wf := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".DS_Store" || filepath.Ext(path) == ".cdx" {
			return nil
		}
		var gz bool
		if filepath.Ext(path) == ".gz" {
			gz = true
		}
		wrdr, wrdr2 := open(path)
		if wrdr == nil {
			return nil
		}
		var count int
		for {
			count++
			r1, err1 := wrdr.Next()
			r2, err2 := wrdr2.Next()
			if err1 != err2 {
				if strings.Index(path, "invalid") != -1 {
					return nil
				}
				t.Fatalf("test case: %s\nunequal errors %v, %v, %d", path, err1, err2, count)
			}
			if err1 != nil {
				break
			}
			if r1.URL() != r2.URL() {
				t.Fatalf("test case: %s\nunequal urls, %s, %s, %d", path, r1.URL(), r2.URL(), count)
			}
			b1, _ := ioutil.ReadAll(r1)
			b2, _ := ioutil.ReadAll(r2)
			if !bytes.Equal(b1, b2) {
				t.Fatalf("test case: %s\nreads aren't equal at %d:\nfirst read:\n%s\n\nsecond read:\n%s\n\n", path, count, string(b1), string(b2))
			}
			if !gz {
				s1, _ := r1.Slice(0, len(b1))
				s2, _ := r1.EofSlice(0, len(b1))
				if !bytes.Equal(b1, s1) || !bytes.Equal(s1, s2) {
					t.Fatalf("test case: %s\nslices aren't equal at %d:\nread:%s\nfirst slice:%s\nsecond slice:%s\n\n", path, count, string(b1), string(s1), string(s2))
				}
			}
		}
		count = 0
		wrdr, wrdr2 = open(path)
		if wrdr == nil {
			return nil
		}
		for {
			count++
			r1, err1 := wrdr.NextPayload()
			r2, err2 := wrdr2.NextPayload()
			if err1 != err2 {
				t.Fatalf("payload test case: %s\nunequal errors %v, %v, %d", path, err1, err2, count)
			}
			if err1 != nil {
				break
			}
			if r1.URL() != r2.URL() {
				t.Fatalf("payload test case: %s\nunequal urls, %s, %s, %d", path, r1.URL(), r2.URL(), count)
			}
			b1, _ := ioutil.ReadAll(r1)
			b2, _ := ioutil.ReadAll(r2)
			if !bytes.Equal(b1, b2) {
				t.Fatalf("payload test case: %s\nreads aren't equal at %d:\nfirst read:\n%s\n\nsecond read:\n%s\n\n", path, count, string(b1), string(b2))
			}
			if !gz {
				s1, _ := r1.Slice(0, len(b1))
				s2, _ := r1.EofSlice(0, len(b1))
				if !bytes.Equal(b1, s1) || !bytes.Equal(s1, s2) {
					t.Fatalf("test case: %s\nslices aren't equal at %d:\nread:%s\nfirst slice:%s\nsecond slice:%s\n\n", path, count, string(b1), string(s1), string(s2))
				}
			}
		}
		return nil
	}
	filepath.Walk("examples", wf)
}
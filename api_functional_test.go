/*
 * Minio Go Library for Amazon S3 Compatible Cloud Storage (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package minio_test

import (
	"bytes"
	crand "crypto/rand"
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz01234569"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func randString(n int, src rand.Source) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b[0:30])
}

func TestRemovePartiallyUploaded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping function tests for short runs")
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Connect and make sure bucket exists.
	c, err := minio.New(
		"s3.amazonaws.com",
		os.Getenv("ACCESS_KEY"),
		os.Getenv("SECRET_KEY"),
		false,
	)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Set user agent.
	c.SetAppInfo("Minio-go-FunctionalTest", "0.1.0")

	// Enable tracing, write to stdout.
	// c.TraceOn(os.Stderr)

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()))

	// make a new bucket.
	err = c.MakeBucket(bucketName, "private", "us-east-1")
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	reader, writer := io.Pipe()
	go func() {
		i := 0
		for i < 25 {
			_, err = io.CopyN(writer, crand.Reader, 128*1024)
			if err != nil {
				t.Fatal("Error:", err, bucketName)
			}
			i++
		}
		writer.CloseWithError(errors.New("Proactively closed to be verified later."))
	}()

	objectName := bucketName + "-resumable"
	_, err = c.PutObject(bucketName, objectName, reader, "application/octet-stream")
	if err == nil {
		t.Fatal("Error: PutObject should fail.")
	}
	if err.Error() != "Proactively closed to be verified later." {
		t.Fatal("Error:", err)
	}
	err = c.RemoveIncompleteUpload(bucketName, objectName)
	if err != nil {
		t.Fatal("Error:", err)
	}
	err = c.RemoveBucket(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}
}

func TestResumableFPutObject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping functional tests for the short runs")
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Connect and make sure bucket exists.
	c, err := minio.New(
		"s3.amazonaws.com",
		os.Getenv("ACCESS_KEY"),
		os.Getenv("SECRET_KEY"),
		false,
	)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Set user agent.
	c.SetAppInfo("Minio-go-FunctionalTest", "0.1.0")

	// Enable tracing, write to stdout.
	// c.TraceOn(os.Stderr)

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()))

	// make a new bucket.
	err = c.MakeBucket(bucketName, "private", "us-east-1")
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	file, err := ioutil.TempFile(os.TempDir(), "resumable")
	if err != nil {
		t.Fatal("Error:", err)
	}

	n, err := io.CopyN(file, crand.Reader, 11*1024*1024)
	if err != nil {
		t.Fatal("Error:", err)
	}
	if n != int64(11*1024*1024) {
		t.Fatalf("Error: number of bytes does not match, want %v, got %v\n", 11*1024*1024, n)
	}

	objectName := bucketName + "-resumable"

	n, err = c.FPutObject(bucketName, objectName, file.Name(), "application/octet-stream")
	if err != nil {
		t.Fatal("Error:", err)
	}
	if n != int64(11*1024*1024) {
		t.Fatalf("Error: number of bytes does not match, want %v, got %v\n", 11*1024*1024, n)
	}

	// Close the file pro-actively for windows.
	file.Close()

	err = c.RemoveObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	err = c.RemoveBucket(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}

	err = os.Remove(file.Name())
	if err != nil {
		t.Fatal("Error:", err)
	}
}

func TestResumablePutObject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping functional tests for the short runs")
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Connect and make sure bucket exists.
	c, err := minio.New(
		"s3.amazonaws.com",
		os.Getenv("ACCESS_KEY"),
		os.Getenv("SECRET_KEY"),
		false,
	)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("Minio-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()))

	// make a new bucket.
	err = c.MakeBucket(bucketName, "private", "us-east-1")
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	// generate 11MB
	buf := make([]byte, 11*1024*1024)

	_, err = io.ReadFull(crand.Reader, buf)
	if err != nil {
		t.Fatal("Error:", err)
	}

	objectName := bucketName + "-resumable"
	reader := bytes.NewReader(buf)
	n, err := c.PutObject(bucketName, objectName, reader, "application/octet-stream")
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}
	if n != int64(len(buf)) {
		t.Fatalf("Error: number of bytes does not match, want %v, got %v\n", len(buf), n)
	}

	err = c.RemoveObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	err = c.RemoveBucket(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}
}

func TestGetObjectReadSeekFunctional(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping functional tests for short runs")
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Connect and make sure bucket exists.
	c, err := minio.New(
		"s3.amazonaws.com",
		os.Getenv("ACCESS_KEY"),
		os.Getenv("SECRET_KEY"),
		false,
	)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("Minio-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()))

	// make a new bucket.
	err = c.MakeBucket(bucketName, "private", "us-east-1")
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	// generate data more than 32K
	buf := make([]byte, rand.Intn(1<<20)+32*1024)

	_, err = io.ReadFull(crand.Reader, buf)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()))
	n, err := c.PutObject(bucketName, objectName, bytes.NewReader(buf), "binary/octet-stream")
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}

	if n != int64(len(buf)) {
		t.Fatalf("Error: number of bytes does not match, want %v, got %v\n", len(buf), n)
	}

	// read the data back
	r, err := c.GetObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}

	st, err := r.Stat()
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}
	if st.Size != int64(len(buf)) {
		t.Fatalf("Error: number of bytes in stat does not match, want %v, got %v\n",
			len(buf), st.Size)
	}

	offset := int64(2048)
	n, err = r.Seek(offset, 0)
	if err != nil {
		t.Fatal("Error:", err, offset)
	}
	if n != offset {
		t.Fatalf("Error: number of bytes seeked does not match, want %v, got %v\n",
			offset, n)
	}
	n, err = r.Seek(0, 1)
	if err != nil {
		t.Fatal("Error:", err)
	}
	if n != offset {
		t.Fatalf("Error: number of current seek does not match, want %v, got %v\n",
			offset, n)
	}
	_, err = r.Seek(offset, 2)
	if err == nil {
		t.Fatal("Error: seek on positive offset for whence '2' should error out")
	}
	n, err = r.Seek(-offset, 2)
	if err != nil {
		t.Fatal("Error:", err)
	}
	if n != 0 {
		t.Fatalf("Error: number of bytes seeked back does not match, want 0, got %v\n", n)
	}
	var buffer bytes.Buffer
	if _, err = io.CopyN(&buffer, r, st.Size); err != nil {
		t.Fatal("Error:", err)
	}
	if !bytes.Equal(buf, buffer.Bytes()) {
		t.Fatal("Error: Incorrect read bytes v/s original buffer.")
	}
	err = c.RemoveObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	err = c.RemoveBucket(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}
}

func TestGetObjectReadAtFunctional(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping functional tests for the short runs")
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Connect and make sure bucket exists.
	c, err := minio.New(
		"s3.amazonaws.com",
		os.Getenv("ACCESS_KEY"),
		os.Getenv("SECRET_KEY"),
		false,
	)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("Minio-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()))

	// make a new bucket.
	err = c.MakeBucket(bucketName, "private", "us-east-1")
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	// generate data more than 32K
	buf := make([]byte, rand.Intn(1<<20)+32*1024)

	_, err = io.ReadFull(crand.Reader, buf)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()))
	n, err := c.PutObject(bucketName, objectName, bytes.NewReader(buf), "binary/octet-stream")
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}

	if n != int64(len(buf)) {
		t.Fatalf("Error: number of bytes does not match, want %v, got %v\n", len(buf), n)
	}

	// read the data back
	r, err := c.GetObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}

	st, err := r.Stat()
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName)
	}
	if st.Size != int64(len(buf)) {
		t.Fatalf("Error: number of bytes in stat does not match, want %v, got %v\n",
			len(buf), st.Size)
	}

	offset := int64(2048)

	// read directly
	buf2 := make([]byte, 512)
	buf3 := make([]byte, 512)
	buf4 := make([]byte, 512)

	m, err := r.ReadAt(buf2, offset)
	if err != nil {
		t.Fatal("Error:", err, st.Size, len(buf2), offset)
	}
	if m != len(buf2) {
		t.Fatalf("Error: ReadAt read shorter bytes before reaching EOF, want %v, got %v\n", m, len(buf2))
	}
	if !bytes.Equal(buf2, buf[offset:offset+512]) {
		t.Fatal("Error: Incorrect read between two ReadAt from same offset.")
	}
	offset += 512
	m, err = r.ReadAt(buf3, offset)
	if err != nil {
		t.Fatal("Error:", err, st.Size, len(buf3), offset)
	}
	if m != len(buf3) {
		t.Fatalf("Error: ReadAt read shorter bytes before reaching EOF, want %v, got %v\n", m, len(buf3))
	}
	if !bytes.Equal(buf3, buf[offset:offset+512]) {
		t.Fatal("Error: Incorrect read between two ReadAt from same offset.")
	}
	offset += 512
	m, err = r.ReadAt(buf4, offset)
	if err != nil {
		t.Fatal("Error:", err, st.Size, len(buf4), offset)
	}
	if m != len(buf4) {
		t.Fatalf("Error: ReadAt read shorter bytes before reaching EOF, want %v, got %v\n", m, len(buf4))
	}
	if !bytes.Equal(buf4, buf[offset:offset+512]) {
		t.Fatal("Error: Incorrect read between two ReadAt from same offset.")
	}

	buf5 := make([]byte, n)
	// Read the whole object.
	m, err = r.ReadAt(buf5, 0)
	if err != nil {
		if err != io.EOF {
			t.Fatal("Error:", err, len(buf5))
		}
	}
	if m != len(buf5) {
		t.Fatalf("Error: ReadAt read shorter bytes before reaching EOF, want %v, got %v\n", m, len(buf5))
	}
	if !bytes.Equal(buf, buf5) {
		t.Fatal("Error: Incorrect data read in GetObject, than what was previously upoaded.")
	}

	buf6 := make([]byte, n+1)
	// Read the whole object and beyond.
	_, err = r.ReadAt(buf6, 0)
	if err != nil {
		if err != io.EOF {
			t.Fatal("Error:", err, len(buf6))
		}
	}
	err = c.RemoveObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	err = c.RemoveBucket(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}
}

func TestFunctional(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping functional tests for the short runs")
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	c, err := minio.New(
		"s3.amazonaws.com",
		os.Getenv("ACCESS_KEY"),
		os.Getenv("SECRET_KEY"),
		false,
	)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Enable to debug
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("Minio-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()))

	// make a new bucket.
	err = c.MakeBucket(bucketName, "private", "us-east-1")
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	// generate a random file name.
	fileName := randString(60, rand.NewSource(time.Now().UnixNano()))
	file, err := os.Create(fileName)
	if err != nil {
		t.Fatal("Error:", err)
	}
	var totalSize int64
	for i := 0; i < 3; i++ {
		buf := make([]byte, rand.Intn(1<<19))
		n, err := file.Write(buf)
		if err != nil {
			t.Fatal("Error:", err)
		}
		totalSize += int64(n)
	}
	file.Close()

	// verify if bucket exits and you have access.
	err = c.BucketExists(bucketName)
	if err != nil {
		t.Fatal("Error:", err, bucketName)
	}

	// make the bucket 'public read/write'.
	err = c.SetBucketACL(bucketName, "public-read-write")
	if err != nil {
		t.Fatal("Error:", err)
	}

	// get the previously set acl.
	acl, err := c.GetBucketACL(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}

	// acl must be 'public read/write'.
	if acl != minio.BucketACL("public-read-write") {
		t.Fatal("Error:", acl)
	}

	// list all buckets.
	buckets, err := c.ListBuckets()
	if len(buckets) == 0 {
		t.Fatal("Error: list buckets cannot be empty", buckets)
	}
	if err != nil {
		t.Fatal("Error:", err)
	}

	// Verify if previously created bucket is listed in list buckets.
	bucketFound := false
	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			bucketFound = true
		}
	}

	// If bucket not found error out.
	if !bucketFound {
		t.Fatal("Error: bucket ", bucketName, "not found")
	}

	objectName := bucketName + "unique"

	// generate data
	buf := make([]byte, rand.Intn(1<<19))
	_, err = io.ReadFull(crand.Reader, buf)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	n, err := c.PutObject(bucketName, objectName, bytes.NewReader(buf), "")
	if err != nil {
		t.Fatal("Error: ", err)
	}
	if n != int64(len(buf)) {
		t.Fatal("Error: bad length ", n, len(buf))
	}

	n, err = c.PutObject(bucketName, objectName+"-nolength", bytes.NewReader(buf), "binary/octet-stream")
	if err != nil {
		t.Fatal("Error:", err, bucketName, objectName+"-nolength")
	}

	if n != int64(len(buf)) {
		t.Fatalf("Error: number of bytes does not match, want %v, got %v\n", len(buf), n)
	}

	newReader, err := c.GetObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	newReadBytes, err := ioutil.ReadAll(newReader)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	if !bytes.Equal(newReadBytes, buf) {
		t.Fatal("Error: bytes mismatch.")
	}

	err = c.FGetObject(bucketName, objectName, fileName+"-f")
	if err != nil {
		t.Fatal("Error: ", err)
	}

	presignedGetURL, err := c.PresignedGetObject(bucketName, objectName, 3600*time.Second)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	resp, err := http.Get(presignedGetURL)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("Error: ", resp.Status)
	}
	newPresignedBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	if !bytes.Equal(newPresignedBytes, buf) {
		t.Fatal("Error: bytes mismatch.")
	}

	presignedPutURL, err := c.PresignedPutObject(bucketName, objectName+"-presigned", 3600*time.Second)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	buf = make([]byte, rand.Intn(1<<20))
	_, err = io.ReadFull(crand.Reader, buf)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	req, err := http.NewRequest("PUT", presignedPutURL, bytes.NewReader(buf))
	if err != nil {
		t.Fatal("Error: ", err)
	}
	httpClient := &http.Client{}
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	newReader, err = c.GetObject(bucketName, objectName+"-presigned")
	if err != nil {
		t.Fatal("Error: ", err)
	}

	newReadBytes, err = ioutil.ReadAll(newReader)
	if err != nil {
		t.Fatal("Error: ", err)
	}

	if !bytes.Equal(newReadBytes, buf) {
		t.Fatal("Error: bytes mismatch.")
	}

	err = c.RemoveObject(bucketName, objectName)
	if err != nil {
		t.Fatal("Error: ", err)
	}
	err = c.RemoveObject(bucketName, objectName+"-f")
	if err != nil {
		t.Fatal("Error: ", err)
	}
	err = c.RemoveObject(bucketName, objectName+"-nolength")
	if err != nil {
		t.Fatal("Error: ", err)
	}
	err = c.RemoveObject(bucketName, objectName+"-presigned")
	if err != nil {
		t.Fatal("Error: ", err)
	}
	err = c.RemoveBucket(bucketName)
	if err != nil {
		t.Fatal("Error:", err)
	}
	err = c.RemoveBucket(bucketName)
	if err == nil {
		t.Fatal("Error:")
	}
	if err.Error() != "The specified bucket does not exist" {
		t.Fatal("Error: ", err)
	}
	if err = os.Remove(fileName); err != nil {
		t.Fatal("Error: ", err)
	}
	if err = os.Remove(fileName + "-f"); err != nil {
		t.Fatal("Error: ", err)
	}
}

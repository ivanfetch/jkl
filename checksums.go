package jkl

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func verifyChecksum(fileName, checksumsFileName string) (verified bool, err error) {
	actualChecksum, err := getSha256Checksum(fileName)
	if err != nil {
		return false, err
	}
	wantChecksum, ok, err := getEntryFromChecksumsFile(checksumsFileName, fileName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("no entry in checksums file %s for file name %s", checksumsFileName, fileName)
	}
	if actualChecksum == wantChecksum {
		debugLog.Printf("checksum verified for %s", fileName)
		return true, nil
	}
	debugLog.Printf("checksum not verified for %s", fileName)
	return false, nil
}

func getSha256Checksum(fileName string) (checksum string, err error) {
	debugLog.Printf("getting sha256 checksum of file %s", fileName)
	f, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	checksum = hex.EncodeToString(h.Sum(nil))
	debugLog.Printf("checksum is: %q", checksum)
	return checksum, nil
}

func getEntryFromChecksumsFile(checksumsFileName, wantChecksumForFileName string) (checksum string, found bool, err error) {
	debugLog.Printf("searching %s for the checksum of %s", checksumsFileName, wantChecksumForFileName)
	f, err := os.Open(checksumsFileName)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	return getChecksumsEntry(f, filepath.Base(wantChecksumForFileName))
}

func getChecksumsEntry(f io.Reader, fileName string) (checksum string, found bool, err error) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		debugLog.Printf("looking at checksums line: %s", line)
		fields := strings.Fields(line) // order is: <checksum> <filename>
		if len(fields) != 2 {
			debugLog.Printf("skipping processing of checksums line because %d is not the expected 2 space-separated fields: %q", len(fields), line)
			continue
		}
		if fields[1] == fileName {
			debugLog.Printf("found checksum %q for file %s", fields[0], fileName)
			return fields[0], true, nil
		}
	}
	err = scanner.Err()
	if err != nil {
		return "", false, err
	}
	return "", false, nil
}

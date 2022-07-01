package jkl

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
)

// fileTypeReader extends io.Reader by providing the file type, determined by
// reading the first 512 bytes.
type fileTypeReader struct {
	io.Reader
	fileType string
}

// NewFileTypeReader returns a fileTypeReader and the file type of the
// supplied io.Reader.
func NewFileTypeReader(f io.Reader) (ftr *fileTypeReader, fileType string, err error) {
	ftr = &fileTypeReader{}
	buffer := make([]byte, 512)
	n, err := f.Read(buffer)
	// Restore; rewind the original os.File before potentially returning from a
	// Read error above.
	resetReader := io.MultiReader(bytes.NewBuffer(buffer[:n]), f)
	ftr.Reader = resetReader
	if errors.Is(err, io.EOF) {
		ftr.fileType = "unknown"
		return ftr, ftr.fileType, nil
	}
	if err != nil {
		return nil, "", err
	}
	contentType, err := filetype.Match(buffer)
	if err != nil {
		return nil, "", err
	}
	ftr.fileType = contentType.Extension
	return ftr, ftr.fileType, nil
}

// Return the file type of the io.Reader.
func (f *fileTypeReader) Type() string {
	return f.fileType
}

// ExtractFile uncompresses and unarchives a file of type gzip, bzip2, tar,
// and zip. If the file is not one of these types, wasExtracted returns
// false.
func ExtractFile(filePath string) (wasExtracted bool, err error) {
	oldCWD, err := os.Getwd()
	if err != nil {
		return false, err
	}
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false, err
	}
	destDirName := filepath.Dir(filePath)
	debugLog.Printf("extracting file %q into directory %q", absFilePath, destDirName)
	err = os.Chdir(destDirName)
	if err != nil {
		return false, err
	}
	defer func() {
		dErr := os.Chdir(oldCWD)
		if dErr != nil { // avoid setting upstream err to nil
			err = dErr
		}
	}()
	f, err := os.Open(absFilePath)
	if err != nil {
		return false, err
	}
	fileStat, err := f.Stat()
	if err != nil {
		return false, err
	}
	fileSize := fileStat.Size()
	ftr, fileType, err := NewFileTypeReader(f)
	if err != nil {
		return false, err
	}
	debugLog.Printf("file type %v\n", fileType)
	fileName := filepath.Base(filePath)
	switch fileType {
	case "gz":
		err := gunzipFile(ftr)
		if err != nil {
			return false, err
		}
	case "bz2":
		err := bunzip2File(ftr, fileName)
		if err != nil {
			return false, err
		}
	case "tar":
		err = extractTarFile(ftr)
		if err != nil {
			return false, err
		}
	case "zip":
		// archive/zip requires io.ReaderAt, satisfied by os.File instead of
		// io.Reader.
		// The unzip pkg explicitly positions the ReaderAt, therefore is not
		// impacted by the fileTypeReader having read the first 512 bytes above.
		err = extractZipFile(f, fileSize)
		if err != nil {
			return false, err
		}
	default:
		debugLog.Printf("nothing to extract from file %s, unknown file type %q", fileName, fileType)
		return false, nil
	}
	return true, nil
}

// saveAs writes the content of an io.Reader to the specified file. If the
// base directory does not exist, it will be created.
func saveAs(r io.Reader, filePath string) error {
	baseDir := filepath.Dir(filePath)
	_, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		debugLog.Printf("creating directory %q", baseDir)
		err := os.MkdirAll(baseDir, 0700)
		if err != nil {
			return err
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Cannot open %s: %v", filePath, err)
	}
	defer f.Close()
	debugLog.Printf("saving to file %s\n", filePath)
	_, err = io.Copy(f, r)
	if err != nil {
		return fmt.Errorf("Cannot write to %s: %v", filePath, err)
	}
	return nil
}

// gunzipFile uses gunzip to decompress the specified io.Reader. If the result
// is a tar file, it will be extracted, otherwise the io.Reader is written to
// a file using saveAs().
func gunzipFile(r io.Reader) error {
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	fileName := gzipReader.Header.Name
	debugLog.Printf("decompressing gzip, optional file name is %q\n", fileName)
	ftr, fileType, err := NewFileTypeReader(gzipReader)
	if err != nil {
		return err
	}
	if fileType == "tar" {
		err := extractTarFile(ftr)
		if err != nil {
			return fmt.Errorf("while extracting ungzipped tar: %v", err)
		}
		return nil
	}
	debugLog.Println("nothing to unarchive, saving direct file.")
	err = saveAs(ftr, fileName)
	if err != nil {
		return err
	}
	return nil
}

// bunzip2File uses bzip2 to decompress the specified io.Reader. If the result
// is a tar file, it will be extracted, otherwise the io.Reader is written to
// a file using saveAs() and the original file name minus the .bz2 extension.
func bunzip2File(r io.Reader, filePath string) error {
	debugLog.Println("decompressing bzip2")
	bzip2Reader := bzip2.NewReader(r)
	baseFileName := strings.TrimSuffix(filepath.Base(filePath), ".bz2")
	baseFileName = strings.TrimSuffix(baseFileName, ".BZ2")
	ftr, fileType, err := NewFileTypeReader(bzip2Reader)
	if err != nil {
		return err
	}
	if fileType == "tar" {
		err := extractTarFile(ftr)
		if err != nil {
			return fmt.Errorf("while extracting bunzip2ed tar: %v", err)
		}
		return nil
	}
	debugLog.Println("nothing to unarchive, saving direct file.")
	err = saveAs(ftr, baseFileName)
	if err != nil {
		return err
	}
	return nil
}

// extractTarFile uses tar to extract the specified io.Reader into the current
// directory.
// Files are extracted in a flat hierarchy, without their sub-directories.
func extractTarFile(r io.Reader) error {
	debugLog.Println("extracting tar")
	tarReader := tar.NewReader(r)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			debugLog.Println("end of tar file")
			break
		}
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			debugLog.Printf("skipping directory %q", header.Name)
			continue
			/* This code kept for future `retainDirStructure` option.
			err = os.Mkdir(header.Name, 0700)
			if err != nil {
				return err
			}
			*/
		case tar.TypeReg:
			err = saveAs(tarReader, filepath.Base(header.Name))
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown file type %q for file %q in tar file", header.Typeflag, header.Name)
		}
	}
	return nil
}

// extractZipFile uses zip to extract the specified os.File into the
// current directory.
// Files are extracted in a flat hierarchy, without their sub-directories.
func extractZipFile(f *os.File, size int64) error {
	debugLog.Println("extracting zip")
	zipReader, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}
	for _, zrf := range zipReader.File {
		if strings.HasSuffix(zrf.Name, "/") {
			debugLog.Printf("Skipping directory %q", zrf.Name)
			continue
		}
		zf, err := zrf.Open()
		if err != nil {
			return fmt.Errorf("cannot open %s in zip file: %v", zrf.Name, err)
		}
		err = saveAs(zf, filepath.Base(zrf.Name))
		if err != nil {
			zf.Close()
			return fmt.Errorf("Cannot write to %s: %v", f.Name(), err)
		}
		zf.Close()
	}
	return nil
}

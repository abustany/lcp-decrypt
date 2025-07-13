package lcp

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"strings"
)

type decryptOptions struct {
	Log func(msg string)
}

type DecryptOption func(*decryptOptions)

func WithLogger(log func(msg string)) DecryptOption {
	return func(o *decryptOptions) {
		o.Log = log
	}
}

type EncryptionAlgorithm string

const (
	EncryptionAlgorithmAES256CBC       EncryptionAlgorithm = "http://www.w3.org/2001/04/xmlenc#aes256-cbc"
	EncryptionAlgorithmFontObfuscation EncryptionAlgorithm = "http://www.idpf.org/2008/embedding"
)

// Decrypt reads an EPUB file encrypted with the Readium LCP DRM from in and
// outputs a regular EPUB file to out.
//
// isSize should be the total size of the input data, and userKeyHex the hex encoded LCP user key.
// Optionally licenseFile is a separate LCP licence file (.lcpl)
func Decrypt(out io.Writer, in io.ReaderAt, inSize int64, userKeyHex string, licenseFile io.Reader, opts ...DecryptOption) error {
	var decryptOptions decryptOptions

	for _, o := range opts {
		o(&decryptOptions)
	}

	log := func(msg string) {
		if decryptOptions.Log == nil {
			return
		}
		decryptOptions.Log(msg)
	}

	if userKeyHex == "" {
		return fmt.Errorf("user key not specified")
	}

	userKey, err := hex.DecodeString(userKeyHex)
	if err != nil {
		return fmt.Errorf("error decoding user key: %w", err)
	}

	inFile, err := zip.NewReader(in, inSize)
	if err != nil {
		return fmt.Errorf("error opening input file: %w", err)
	}

	if licenseFile == nil {
		tempLicenseFile, err := inFile.Open("META-INF/license.lcpl")
		if err != nil {
		        return fmt.Errorf("error opening license file: %w", err)
	        }
		licenseFile = tempLicenseFile
	}

	contentKey, err := getContentKey(licenseFile, userKey)
	if err != nil {
		return fmt.Errorf("error getting content key: %w", err)
	}

	encryptedFiles, err := listEncryptedFiles(inFile)
	if err != nil {
		return fmt.Errorf("error listing encrypted files: %w", err)
	}

	outZip := zip.NewWriter(out)

	if err := outZip.SetComment(inFile.Comment); err != nil {
		return fmt.Errorf("error setting output file comment: %w", err)
	}

	encryptedFilesSet := groupFileEntriesByPath(encryptedFiles)

	// According to the ePUB spec, the "mimetype" file must come first in the
	// archive and not be compressed.
	mimetypeFile, err := outZip.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})

	if _, err := io.WriteString(mimetypeFile, "application/epub+zip"); err != nil {
		return fmt.Errorf("error appending mimetype file to output zip file: %w", err)
	}

	for _, f := range inFile.File {
		switch f.Name {
		case "META-INF/encryption.xml", "META-INF/license.lcpl", "mimetype":
			continue // already written / not needed once content is decrypted
		}

		log("Processing file " + f.Name + "...")

		dstFile, err := outZip.Create(f.Name)
		if err != nil {
			return fmt.Errorf("error appending file %s to output zip file: %w", f.Name, err)
		}

		if strings.HasSuffix(f.Name, "/") {
			continue // no need to copy any data for directories
		}

		srcFile, err := f.Open()
		if err != nil {
			return fmt.Errorf("error opening file %s from input zip file: %w", f.Name, err)
		}

		if fileEntry, ok := encryptedFilesSet[f.Name]; ok {
			err = decryptFile(dstFile, srcFile, contentKey, fileEntry.EncryptionAlgorithm, fileEntry.IsCompressed)
		} else {
			_, err = io.Copy(dstFile, srcFile)
		}

		if err != nil {
			return fmt.Errorf("error copying data for file %s to output zip file: %w", f.Name, err)
		}

		if err := srcFile.Close(); err != nil {
			return fmt.Errorf("error closing file %s from input zip file: %w", f.Name, err)
		}
	}

	if err := outZip.Close(); err != nil {
		return fmt.Errorf("error finalizing output zip file: %w", err)
	}

	log("Decrypted ePUB")

	return nil
}

type FileEntry struct {
	Path                string
	IsCompressed        bool
	EncryptionAlgorithm EncryptionAlgorithm
}

func listEncryptedFiles(epubRoot fs.FS) ([]FileEntry, error) {
	encFile, err := epubRoot.Open("META-INF/encryption.xml")
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	defer encFile.Close()

	var encryption struct {
		EncryptedData []struct {
			EncryptionMethod struct {
				Algorithm string `xml:"Algorithm,attr"`
			}
			CipherData struct {
				CipherReference struct {
					URI string `xml:"URI,attr"`
				}
			}
			EncryptionProperties struct {
				EncryptionProperty []struct {
					Compression []struct {
						Method int `xml:"Method,attr"`
					}
				}
			}
		}
	}

	if err := xml.NewDecoder(encFile).Decode(&encryption); err != nil {
		return nil, fmt.Errorf("error decoding file: %w", err)
	}

	var res []FileEntry

	for _, d := range encryption.EncryptedData {
		path, err := url.PathUnescape(d.CipherData.CipherReference.URI)
		if err != nil {
			return nil, fmt.Errorf("error decoding entry path %q: %w", d.CipherData.CipherReference.URI, err)
		}

		isCompressed := false
		var encryptionAlgorithm EncryptionAlgorithm

		switch d.EncryptionMethod.Algorithm {
		case string(EncryptionAlgorithmAES256CBC), string(EncryptionAlgorithmFontObfuscation):
			encryptionAlgorithm = EncryptionAlgorithm(d.EncryptionMethod.Algorithm)
		default:
			return nil, fmt.Errorf("unsupported encryption algorithm for file %s: %s", path, d.EncryptionMethod.Algorithm)
		}

	PropertyLoop:
		for _, p := range d.EncryptionProperties.EncryptionProperty {
			for _, c := range p.Compression {
				if c.Method == 8 {
					isCompressed = true
					break PropertyLoop
				}
			}
		}

		res = append(res, FileEntry{
			Path:                path,
			IsCompressed:        isCompressed,
			EncryptionAlgorithm: encryptionAlgorithm,
		})
	}

	return res, nil
}

func groupFileEntriesByPath(strs []FileEntry) map[string]FileEntry {
	res := make(map[string]FileEntry, len(strs))

	for _, s := range strs {
		res[s.Path] = s
	}

	return res
}

func getContentKey(licenseFile io.Reader, userKey []byte) ([]byte, error) {
	var license struct {
		ID         string `json:"id"`
		Encryption struct {
			ContentKey struct {
				EncryptedValue string `json:"encrypted_value"`
			} `json:"content_key"`
			UserKey struct {
				KeyCheck string `json:"key_check"`
			} `json:"user_key"`
		}
	}

	if err := json.NewDecoder(licenseFile).Decode(&license); err != nil {
		return nil, fmt.Errorf("error decoding json: %w", err)
	}

	encryptedKeyCheck, err := base64.StdEncoding.DecodeString(license.Encryption.UserKey.KeyCheck)
	if err != nil {
		return nil, fmt.Errorf("error decoding key check: %w", err)
	}

	keyCheck, err := decipherAES256CBC(encryptedKeyCheck, userKey)
	if err != nil {
		return nil, fmt.Errorf("error decrypting key check: %w", err)
	}

	if string(keyCheck) != license.ID {
		return nil, fmt.Errorf("decrypted key check (%s) does not match license ID (%s)", keyCheck, license.ID)
	}

	encryptedContentKey, err := base64.StdEncoding.DecodeString(license.Encryption.ContentKey.EncryptedValue)
	if err != nil {
		return nil, fmt.Errorf("error decoding content key: %w", err)
	}

	contentKey, err := decipherAES256CBC(encryptedContentKey, userKey)
	if err != nil {
		return nil, fmt.Errorf("error decrypting content key: %w", err)
	}

	return contentKey, nil
}

func decipherAES256CBC(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("error creating cipher: %w", err)
	}

	iv, cipherData := data[:aes.BlockSize], data[aes.BlockSize:]

	if len(data) == 0 {
		return nil, nil
	}

	res := make([]byte, len(cipherData))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(res, cipherData)

	paddingLen := int(res[len(res)-1])
	if paddingLen > len(res) {
		return nil, fmt.Errorf("invalid padding length %d (data length is %d)", paddingLen, len(res))
	}

	res = res[:len(res)-paddingLen]

	return res, nil
}

func decipherFontObfuscation(data, key []byte) ([]byte, error) {
	// Let's assume readers know how to deal with this algorithm... Worst case,
	// let's hope they fallback to any font.
	return data, nil
}

func decryptFile(dst io.Writer, src io.Reader, contentKey []byte, encryptionAlgorithm EncryptionAlgorithm, isCompressed bool) error {
	encryptedData, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("error reading data: %w", err)
	}

	var decipherFunc func(data []byte, key []byte) (res []byte, err error)

	switch encryptionAlgorithm {
	case EncryptionAlgorithmAES256CBC:
		decipherFunc = decipherAES256CBC
	case EncryptionAlgorithmFontObfuscation:
		decipherFunc = decipherFontObfuscation
	default:
		return fmt.Errorf("invalid encryption algorithm: %s", encryptionAlgorithm)
	}

	data, err := decipherFunc(encryptedData, contentKey)
	if err != nil {
		return fmt.Errorf("error decrypting data: %w", err)
	}

	cleartextReader := io.NopCloser(bytes.NewReader(data))
	defer cleartextReader.Close()

	if isCompressed {
		cleartextReader = flate.NewReader(cleartextReader)
	}

	if _, err := io.Copy(dst, cleartextReader); err != nil {
		return fmt.Errorf("error writing data: %w", err)
	}

	return nil
}

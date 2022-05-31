package main

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %s", err)
	}
}

func run() error {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s -userKey USER_KEY_HEX in.epub out.epub

Decrypts the files of an EPUB book protected with Readium LCP (CARE) DRM. This
program requires the "user key" to operate, in other words it does not "crack"
any DRM. It only decrypts files for which you already have the decryption key.

To obtain the user key, you can for example use mitmproxy with your EPUB reader
application. The app should do a request that looks like

GET https://api.your-book-store.com/v1/lcp/keys/user?device_id=XXX

and the response should look like

[{"user_key": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}]

The 0123... string is the value you should pass in -userKey.
`, os.Args[0])
		flag.PrintDefaults()
	}

	userKeyHex := flag.String("userKey", "", "hex encoded LCP user key")

	flag.Parse()

	inFilename := flag.Arg(0)
	if inFilename == "" {
		return fmt.Errorf("no input file specified")
	}

	outFilename := flag.Arg(1)
	if outFilename == "" {
		return fmt.Errorf("no output file specified")
	}

	if *userKeyHex == "" {
		return fmt.Errorf("user key not specified")
	}

	userKey, err := hex.DecodeString(*userKeyHex)
	if err != nil {
		return fmt.Errorf("error decoding user key: %w", err)
	}

	inFile, err := zip.OpenReader(inFilename)
	if err != nil {
		return fmt.Errorf("error opening input file: %w", err)
	}

	defer inFile.Close()

	contentKey, err := getContentKey(inFile, userKey)
	if err != nil {
		return fmt.Errorf("error getting content key: %w", err)
	}

	encryptedFiles, err := listEncryptedFiles(inFile)
	if err != nil {
		return fmt.Errorf("error listing encrypted files: %w", err)
	}

	outFd, err := os.Create(outFilename)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}

	defer outFd.Close()

	outZip := zip.NewWriter(outFd)

	if err := outZip.SetComment(inFile.Comment); err != nil {
		return fmt.Errorf("error setting output file comment: %w", err)
	}

	encryptedFilesSet := stringSet(encryptedFiles)

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

		log.Printf("Processing file %s...", f.Name)

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

		if _, ok := encryptedFilesSet[f.Name]; ok {
			err = decryptFile(dstFile, srcFile, contentKey)
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

	log.Printf("Decrypted ePUB into %s", outFilename)

	return nil
}

func listEncryptedFiles(epubRoot fs.FS) ([]string, error) {
	encFile, err := epubRoot.Open("META-INF/encryption.xml")
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	defer encFile.Close()

	var encryption struct {
		EncryptedData []struct {
			CipherData struct {
				CipherReference struct {
					URI string `xml:"URI,attr"`
				}
			}
		}
	}

	if err := xml.NewDecoder(encFile).Decode(&encryption); err != nil {
		return nil, fmt.Errorf("error decoding file: %w", err)
	}

	var res []string

	for _, d := range encryption.EncryptedData {
		res = append(res, d.CipherData.CipherReference.URI)
	}

	return res, nil
}

func stringSet(strs []string) map[string]struct{} {
	res := make(map[string]struct{}, len(strs))

	for _, s := range strs {
		res[s] = struct{}{}
	}

	return res
}

func getContentKey(epubRoot fs.FS, userKey []byte) ([]byte, error) {
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

	licenseFile, err := epubRoot.Open("META-INF/license.lcpl")
	if err != nil {
		return nil, fmt.Errorf("error opening license file: %w", err)
	}

	if err := json.NewDecoder(licenseFile).Decode(&license); err != nil {
		return nil, fmt.Errorf("error decoding json: %w", err)
	}

	encryptedKeyCheck, err := base64.StdEncoding.DecodeString(license.Encryption.UserKey.KeyCheck)
	if err != nil {
		return nil, fmt.Errorf("error decoding key check: %w", err)
	}

	keyCheck, err := decipher(encryptedKeyCheck, userKey)
	if err != nil {
		return nil, fmt.Errorf("error decrypting key check")
	}

	if string(keyCheck) != license.ID {
		return nil, fmt.Errorf("decrypted key check (%s) does not match license ID (%s)", keyCheck, license.ID)
	}

	encryptedContentKey, err := base64.StdEncoding.DecodeString(license.Encryption.ContentKey.EncryptedValue)
	if err != nil {
		return nil, fmt.Errorf("error decoding content key: %w", err)
	}

	contentKey, err := decipher(encryptedContentKey, userKey)
	if err != nil {
		return nil, fmt.Errorf("error decrypting content key: %w", err)
	}

	return contentKey, nil
}

func decipher(data, key []byte) ([]byte, error) {
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

func decryptFile(dst io.Writer, src io.Reader, contentKey []byte) error {
	encryptedData, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("error reading data: %w", err)
	}

	data, err := decipher(encryptedData, contentKey)
	if err != nil {
		return fmt.Errorf("error decrypting data: %w", err)
	}

	if _, err := dst.Write(data); err != nil {
		return fmt.Errorf("error writing data: %w", err)
	}

	return nil
}

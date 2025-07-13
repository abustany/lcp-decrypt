package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/abustany/lcp-decrypt/pkg/lcp"
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

The licenceFile argument refers to the .lcpl license file and is optional.
With personalised downloads the license file is embedded in the EPUB file.
`, os.Args[0])
		flag.PrintDefaults()
	}

	userKeyHex := flag.String("userKey", "", "hex encoded LCP user key")
	licenseFileName := flag.String("licenseFile", "", "name of the license file")

	flag.Parse()

	inFilename := flag.Arg(0)
	if inFilename == "" {
		return fmt.Errorf("no input file specified")
	}

	outFilename := flag.Arg(1)
	if outFilename == "" {
		return fmt.Errorf("no output file specified")
	}

	inFd, err := os.Open(inFilename)
	if err != nil {
		return fmt.Errorf("error opening input file: %w", err)
	}

	defer inFd.Close()

	inStat, err := inFd.Stat()
	if err != nil {
		return fmt.Errorf("error stating input file: %w", err)
	}

	outFd, err := os.Create(outFilename)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}

	defer outFd.Close()
	
	var licenseFd *os.File
	if *licenseFileName != "" {
	        licenseFdi, err := os.Open(*licenseFileName)
	        if err != nil {
		        return fmt.Errorf("error opening license file: %w", err)
	        }
		
		licenseFd = licenseFdi
		
	        defer licenseFd.Close()
        }

        if err := lcp.Decrypt(outFd, inFd, inStat.Size(), *userKeyHex, licenseFd, lcp.WithLogger(func(msg string) { log.Println(msg) })); err != nil {
		_ = os.Remove(outFilename) // ignore error here
		return fmt.Errorf("error decrypting file: %w", err)
	}

	if err := outFd.Sync(); err != nil {
		return fmt.Errorf("error flushing output file: %w", err)
	}

	return nil
}

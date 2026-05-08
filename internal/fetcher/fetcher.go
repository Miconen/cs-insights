package fetcher

import (
	"compress/bzip2"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FetchRecentMatches fetches the last 'limit' matches from the user's GCPD page.
// steamID can be a custom URL (e.g., "Miconen") or a 64-bit Steam ID (e.g., "76561198...").
func FetchRecentMatches(steamID, cookie string, limit int, outputDir string) ([]string, error) {
	// Determine if it's a profile ID or custom URL
	urlType := "id"
	if len(steamID) == 17 && isNumeric(steamID) {
		urlType = "profiles"
	}

	url := fmt.Sprintf("https://steamcommunity.com/%s/%s/gcpd/730?tab=matchhistorypremier", urlType, steamID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Cookie", fmt.Sprintf("steamLoginSecure=%s", cookie))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GCPD page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch GCPD page, status code: %d (check your steamLoginSecure cookie)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Regex to find replay download links
	re := regexp.MustCompile(`https?://replay\S+\.dem\.bz2`)
	matches := re.FindAllString(string(body), -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no match downloads found on the page. Ensure your cookie is valid and you have premier matches")
	}

	// Remove duplicates (sometimes the same match has multiple download buttons on the page)
	uniqueLinks := make([]string, 0)
	seen := make(map[string]bool)
	for _, match := range matches {
		if !seen[match] {
			uniqueLinks = append(uniqueLinks, match)
			seen[match] = true
		}
	}

	if limit > len(uniqueLinks) {
		limit = len(uniqueLinks)
	}

	linksToDownload := uniqueLinks[:limit]
	
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	var downloadedFiles []string

	for i, link := range linksToDownload {
		log.Printf("Downloading match %d/%d: %s", i+1, limit, link)
		
		fileName := filepath.Base(link)
		bz2Path := filepath.Join(outputDir, fileName)
		demPath := strings.TrimSuffix(bz2Path, ".bz2")

		// Skip if already downloaded and decompressed
		if _, err := os.Stat(demPath); err == nil {
			log.Printf("Demo already exists at %s, skipping download.", demPath)
			downloadedFiles = append(downloadedFiles, demPath)
			continue
		}

		err := downloadFile(link, bz2Path)
		if err != nil {
			log.Printf("Warning: failed to download %s: %v", link, err)
			continue
		}

		log.Printf("Decompressing %s...", bz2Path)
		err = decompressBz2(bz2Path, demPath)
		if err != nil {
			log.Printf("Warning: failed to decompress %s: %v", bz2Path, err)
			os.Remove(bz2Path) // Cleanup bad download
			continue
		}

		// Cleanup the compressed file to save space
		os.Remove(bz2Path)
		
		downloadedFiles = append(downloadedFiles, demPath)
	}

	return downloadedFiles, nil
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func downloadFile(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func decompressBz2(compressedPath string, decompressedPath string) error {
	compressedFile, err := os.Open(compressedPath)
	if err != nil {
		return err
	}
	defer compressedFile.Close()

	bz2Reader := bzip2.NewReader(compressedFile)

	decompressedFile, err := os.Create(decompressedPath)
	if err != nil {
		return err
	}
	defer decompressedFile.Close()

	_, err = io.Copy(decompressedFile, bz2Reader)
	return err
}

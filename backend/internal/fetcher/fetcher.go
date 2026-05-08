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

type MatchInfo struct {
	Link       string `json:"link"`
	FileName   string `json:"file_name"`
	Downloaded bool   `json:"downloaded"`
	Processed  bool   `json:"processed"` // Harder to check without DB, but let's leave it for API to fill
}

// GetMatchHistory just returns the links found on the page
func GetMatchHistory(steamID, cookie string, outputDir string) ([]MatchInfo, error) {
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
		return nil, fmt.Errorf("failed to fetch GCPD page, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	re := regexp.MustCompile(`https?://replay\S+\.dem\.bz2`)
	matches := re.FindAllString(string(body), -1)

	uniqueLinks := make([]string, 0)
	seen := make(map[string]bool)
	for _, match := range matches {
		if !seen[match] {
			uniqueLinks = append(uniqueLinks, match)
			seen[match] = true
		}
	}

	var matchInfos []MatchInfo
	for _, link := range uniqueLinks {
		fileName := filepath.Base(link)
		bz2Path := filepath.Join(outputDir, fileName)
		demPath := strings.TrimSuffix(bz2Path, ".bz2")
		
		_, err := os.Stat(demPath)
		downloaded := err == nil
		
		matchInfos = append(matchInfos, MatchInfo{
			Link:       link,
			FileName:   fileName,
			Downloaded: downloaded,
		})
	}

	return matchInfos, nil
}

func DownloadAndDecompress(link string, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	fileName := filepath.Base(link)
	bz2Path := filepath.Join(outputDir, fileName)
	demPath := strings.TrimSuffix(bz2Path, ".bz2")

	if _, err := os.Stat(demPath); err == nil {
		return demPath, nil // Already exists
	}

	err := downloadFile(link, bz2Path)
	if err != nil {
		return "", fmt.Errorf("download failed: %v", err)
	}

	err = decompressBz2(bz2Path, demPath)
	if err != nil {
		os.Remove(bz2Path)
		return "", fmt.Errorf("decompression failed: %v", err)
	}

	os.Remove(bz2Path)
	return demPath, nil
}

// FetchRecentMatches keeps the original CLI behavior
func FetchRecentMatches(steamID, cookie string, limit int, outputDir string) ([]string, error) {
	matches, err := GetMatchHistory(steamID, cookie, outputDir)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no match downloads found")
	}

	if limit > len(matches) {
		limit = len(matches)
	}

	var downloadedFiles []string
	for i := 0; i < limit; i++ {
		link := matches[i].Link
		log.Printf("Downloading match %d/%d: %s", i+1, limit, link)
		
		demPath, err := DownloadAndDecompress(link, outputDir)
		if err != nil {
			log.Printf("Warning: failed to process %s: %v", link, err)
			continue
		}
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

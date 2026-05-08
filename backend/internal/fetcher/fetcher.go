package fetcher

import (
	"compress/bzip2"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const shareCodeDictionary = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

type MatchInfo struct {
	Link       string `json:"link"`
	FileName   string `json:"file_name"`
	Downloaded bool   `json:"downloaded"`
	Processed  bool   `json:"processed"` // Harder to check without DB, but let's leave it for API to fill
}

type ShareCodeInfo struct {
	ShareCode  string `json:"share_code"`
	MatchID    string `json:"match_id"`
	OutcomeID  string `json:"outcome_id"`
	TVPort     uint16 `json:"tv_port"`
	DemoURL    string `json:"demo_url"`
	FileName   string `json:"file_name"`
	Downloaded bool   `json:"downloaded"`
	Processed  bool   `json:"processed"`
}

type MatchShareCode struct {
	MatchID   *big.Int
	OutcomeID *big.Int
	TVPort    uint16
}

type nextMatchSharingCodeResponse struct {
	Result struct {
		NextCode string `json:"nextcode"`
	} `json:"result"`
}

// GetNextMatchShareCodes uses Valve's official, narrow-scope match-history API.
// It requires a Steam Web API key, SteamID64, the user's CS match-history auth code
// (called steamidkey by the endpoint), and a known match share code to page from.
func GetNextMatchShareCodes(apiKey, steamID64, authCode, knownCode string, limit int, outputDir string) ([]ShareCodeInfo, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	codes := make([]ShareCodeInfo, 0, limit)
	currentKnownCode := knownCode
	client := &http.Client{}

	for len(codes) < limit {
		values := url.Values{}
		values.Set("key", apiKey)
		values.Set("steamid", steamID64)
		values.Set("steamidkey", authCode)
		values.Set("knowncode", currentKnownCode)

		endpoint := "https://api.steampowered.com/ICSGOPlayers_730/GetNextMatchSharingCode/v1/?" + values.Encode()
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch next sharing code: %v", err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read Steam API response: %v", readErr)
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Steam API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var parsed nextMatchSharingCodeResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse Steam API response: %v", err)
		}

		nextCode := strings.TrimSpace(parsed.Result.NextCode)
		if nextCode == "" || nextCode == "n/a" || nextCode == currentKnownCode {
			break
		}

		info, err := BuildShareCodeInfo(nextCode, outputDir)
		if err != nil {
			return nil, fmt.Errorf("failed to decode share code %s: %v", nextCode, err)
		}

		codes = append(codes, info)
		currentKnownCode = nextCode
	}

	return codes, nil
}

func BuildShareCodeInfo(shareCode string, outputDir string) (ShareCodeInfo, error) {
	decoded, err := DecodeMatchShareCode(shareCode)
	if err != nil {
		return ShareCodeInfo{}, err
	}

	demoURL := DemoURLFromShareCode(decoded)
	fileName := filepath.Base(demoURL)
	demPath := strings.TrimSuffix(filepath.Join(outputDir, fileName), ".bz2")
	_, statErr := os.Stat(demPath)

	return ShareCodeInfo{
		ShareCode:  shareCode,
		MatchID:    decoded.MatchID.String(),
		OutcomeID:  decoded.OutcomeID.String(),
		TVPort:     decoded.TVPort,
		DemoURL:    demoURL,
		FileName:   fileName,
		Downloaded: statErr == nil,
	}, nil
}

func DecodeMatchShareCode(shareCode string) (MatchShareCode, error) {
	cleaned := strings.ReplaceAll(strings.ReplaceAll(shareCode, "CSGO", ""), "-", "")
	if len(cleaned) != 25 {
		return MatchShareCode{}, fmt.Errorf("invalid share code length")
	}

	total := big.NewInt(0)
	base := big.NewInt(int64(len(shareCodeDictionary)))
	for i := len(cleaned) - 1; i >= 0; i-- {
		idx := strings.IndexByte(shareCodeDictionary, cleaned[i])
		if idx < 0 {
			return MatchShareCode{}, fmt.Errorf("invalid share code character %q", cleaned[i])
		}
		total.Mul(total, base)
		total.Add(total, big.NewInt(int64(idx)))
	}

	bytes := total.Bytes()
	if len(bytes) > 18 {
		return MatchShareCode{}, fmt.Errorf("decoded share code is too large")
	}

	padded := make([]byte, 18)
	copy(padded[18-len(bytes):], bytes)

	matchID := littleEndianBigInt(padded[0:8])
	outcomeID := littleEndianBigInt(padded[8:16])
	tvPort := uint16(padded[16]) | uint16(padded[17])<<8

	return MatchShareCode{
		MatchID:   matchID,
		OutcomeID: outcomeID,
		TVPort:    tvPort,
	}, nil
}

func littleEndianBigInt(bytes []byte) *big.Int {
	reversed := make([]byte, len(bytes))
	for i := range bytes {
		reversed[len(bytes)-1-i] = bytes[i]
	}
	return new(big.Int).SetBytes(reversed)
}

func DemoURLFromShareCode(decoded MatchShareCode) string {
	return fmt.Sprintf("https://replay%d.valve.net/730/%s_%s.dem.bz2", decoded.TVPort, decoded.MatchID.String(), decoded.OutcomeID.String())
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
		if strings.HasPrefix(url, "https://replay") {
			return downloadFile(strings.Replace(url, "https://", "http://", 1), filepath)
		}
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

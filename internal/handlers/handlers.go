package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mitchellh/mapstructure"

	"github.com/drewfoos/league-finder/internal/utils"
)

type CacheItem struct {
	Data      map[string]interface{}
	Timestamp time.Time
}

// Define a global cache with a mutex for thread-safety
var leagueDataCache = make(map[string]CacheItem)
var cacheMutex sync.RWMutex
var cacheExpiryDuration = time.Hour * 24 // set desired cache expiry duration

var httpClient = &http.Client{
	Timeout: time.Second * 10, // set desired timeout
}

var regionMapping = map[string]string{
	"BR":   "br1.api.riotgames.com",
	"EUNE": "eun1.api.riotgames.com",
	"EUW":  "euw1.api.riotgames.com",
	"JP":   "jp1.api.riotgames.com",
	"KR":   "kr.api.riotgames.com",
	"LAN":  "la1.api.riotgames.com",
	"LAS":  "la2.api.riotgames.com",
	"NA":   "na1.api.riotgames.com",
	"OCE":  "oc1.api.riotgames.com",
	"TR":   "tr1.api.riotgames.com",
	"RU":   "ru.api.riotgames.com",
	"PH":   "ph2.api.riotgames.com",
	"SG":   "sg2.api.riotgames.com",
	"TH":   "th2.api.riotgames.com",
	"TW":   "tw2.api.riotgames.com",
	"VN":   "vn2.api.riotgames.com",
}

type UrlRequest struct {
	Url    string `json:"url"`
	Region string `json:"region"` // New field to capture region value
}

type SummonerData struct {
	Name          string `json:"name"`
	SummonerLevel int    `json:"summonerLevel"`
	ProfileIconId int    `json:"profileIconId"`
	PUUID         string `json:"puuid"`
}

func writeErrorResponse(w http.ResponseWriter, logMsg string, httpMsg string, code int) {
	log.Println(logMsg)
	http.Error(w, httpMsg, code)
}

func getSummonerData(endpoint, apiKey string) (*SummonerData, error) {
	constructedUrl := fmt.Sprintf("%s?api_key=%s", endpoint, apiKey)
	apiResponse, err := http.Get(constructedUrl)
	if err != nil {
		return nil, err
	}
	defer apiResponse.Body.Close()

	if apiResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Riot API returned status code: %d", apiResponse.StatusCode)
	}

	responseData, err := io.ReadAll(apiResponse.Body)
	if err != nil {
		return nil, err
	}

	var summoner SummonerData
	if err := json.Unmarshal(responseData, &summoner); err != nil {
		return nil, err
	}
	return &summoner, nil
}

func SearchHandler(c *fiber.Ctx) error {
	apiKey := strings.TrimSpace(utils.GetApiKeyFromFile(".environment.env"))
	if apiKey == "" {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	body := c.Body()
	var request UrlRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Failed to parse request body")
	}

	riotBaseUrl, exists := regionMapping[request.Region]
	if !exists {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid region")
	}

	summoner, err := getSummonerData(fmt.Sprintf("https://%s%s", riotBaseUrl, strings.TrimSpace(request.Url)), apiKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch summoner data")
	}

	matchIDs, err := fetchMatchesByPUUID(summoner.PUUID, apiKey, request.Region)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch matches")
	}

	if len(matchIDs) == 0 {
		return c.JSON(summoner)
	}

	participantDataList, err := fetchAndProcessMatchDetailsForIDs(matchIDs, apiKey, summoner.PUUID, request.Region)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch match details")
	}

	return c.JSON(participantDataList)
}

func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	responseJSON, err := json.Marshal(data)
	if err != nil {
		writeErrorResponse(w, "Error marshalling JSON response: "+err.Error(), "Failed to prepare response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

var platformMapping = map[string]string{
	"AMERICAS": "americas.api.riotgames.com",
	"ASIA":     "asia.api.riotgames.com",
	"EUROPE":   "europe.api.riotgames.com",
	"SEA":      "sea.api.riotgames.com",
}

func getPlatformForRegion(region string) string {
	// This map can be expanded based on further requirements
	regionToPlatform := map[string]string{
		"BR":   "AMERICAS",
		"NA":   "AMERICAS",
		"LAN":  "AMERICAS",
		"LAS":  "AMERICAS",
		"OCE":  "AMERICAS",
		"KR":   "ASIA",
		"JP":   "ASIA",
		"PH":   "ASIA",
		"SG":   "ASIA",
		"TH":   "ASIA",
		"TW":   "ASIA",
		"VN":   "ASIA",
		"EUW":  "EUROPE",
		"EUNE": "EUROPE",
		"TR":   "EUROPE",
		"RU":   "EUROPE",
	}

	platform, exists := regionToPlatform[region]
	if !exists {
		return "" // or some default value
	}

	domain, ok := platformMapping[platform]
	if !ok {
		return "" // or some default value
	}

	return domain
}

const baseURLFormatForMatches = "https://%s/lol/match/v5/matches/by-puuid/%s/ids?start=0&count=10&api_key=%s"

func fetchMatchesByPUUID(puuid string, apiKey string, region string) ([]string, error) {
	// Construct the URL to fetch match IDs
	url := fmt.Sprintf(baseURLFormatForMatches, getPlatformForRegion(region), strings.TrimSpace(puuid), strings.TrimSpace(apiKey))

	response, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching matches by PUUID %s: %v", puuid, err)
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Riot API returned non-200 status code %d while fetching matches by PUUID %s", response.StatusCode, puuid)
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	var matchIDs []string
	if err := json.NewDecoder(response.Body).Decode(&matchIDs); err != nil {
		log.Printf("Error decoding JSON response from Riot API for PUUID %s: %v", puuid, err)
		return nil, err
	}

	//log.Printf("Fetched match IDs for PUUID %s: %v", puuid, matchIDs)
	return matchIDs, nil
}

// Fetch detailed match data for a given match ID
const baseURLFormat = "https://%s/lol/match/v5/matches/%s?api_key=%s"

func fetchMatchDetails(matchID string, apiKey string, region string) (map[string]interface{}, error) {
	// Construct the URL to fetch detailed match data
	url := fmt.Sprintf(baseURLFormat, getPlatformForRegion(region), strings.TrimSpace(matchID), strings.TrimSpace(apiKey))

	response, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching match details for match ID %s: %v", matchID, err)
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Riot API returned non-200 status code %d while fetching match details for match ID %s", response.StatusCode, matchID)
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	var matchData map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&matchData); err != nil {
		log.Printf("Error decoding match details from Riot API for match ID %s: %v", matchID, err)
		return nil, err
	}

	return matchData, nil
}

type ParticipantDataList struct {
	Data []ParticipantData `json:"data"`
}

func fetchAndProcessMatchDetailsForIDs(matchIDs []string, apiKey string, puuid string, region string) (ParticipantDataList, error) {
	var dataList ParticipantDataList
	dataList.Data = make([]ParticipantData, 0, len(matchIDs)) // preallocate with expected length

	var wg sync.WaitGroup
	errCh := make(chan string, len(matchIDs)) // error channel

	sem := make(chan struct{}, 10) // adjust based on rate limits or desired concurrency

	for _, matchID := range matchIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire a token
			defer func() { <-sem }() // Release the token after the goroutine completes

			matchData, err := fetchMatchDetails(id, apiKey, region)
			if err != nil {
				errCh <- "Error fetching details for match ID: " + id + " - " + err.Error()
				return
			}

			metadata, exists := matchData["metadata"].(map[string]interface{})
			if !exists {
				errCh <- "metadata doesn't exist for match ID: " + id
				return
			}

			participants, exists := metadata["participants"].([]interface{})
			if !exists {
				errCh <- "No participants array in metadata for match ID: " + id
				return
			}

			for index, participant := range participants {
				participantPUUID, ok := participant.(string)
				if ok && participantPUUID == puuid {
					participantData, err := ExtractParticipantDataAtIndex(matchData, apiKey, region, index)
					if err != nil {
						errCh <- "Error extracting participant data for match ID: " + id + " - " + err.Error()
						return
					}

					dataList.Data = append(dataList.Data, *participantData)
					return
				}
			}
			errCh <- "Couldn't find participant with the given PUUID in the match data for match ID: " + id

		}(matchID)
	}

	go func() {
		wg.Wait()
		close(errCh) // close error channel once all goroutines are done
	}()

	// Log errors concurrently
	for err := range errCh {
		log.Println(err)
	}

	return dataList, nil
}

type ParticipantData struct {
	Assists                        int    `json:"assists"`
	BaronKills                     int    `json:"baronKills"`
	BountyLevel                    int    `json:"bountyLevel"`
	ChampExperience                int    `json:"champExperience"`
	ChampLevel                     int    `json:"champLevel"`
	ChampionId                     int    `json:"championId"`
	ChampionName                   string `json:"championName"`
	ChampionTransform              int    `json:"championTransform"`
	ConsumablesPurchased           int    `json:"consumablesPurchased"`
	DamageDealtToBuildings         int    `json:"damageDealtToBuildings"`
	DamageDealtToObjectives        int    `json:"damageDealtToObjectives"`
	DamageDealtToTurrets           int    `json:"damageDealtToTurrets"`
	DamageSelfMitigated            int    `json:"damageSelfMitigated"`
	Deaths                         int    `json:"deaths"`
	DetectorWardsPlaced            int    `json:"detectorWardsPlaced"`
	DoubleKills                    int    `json:"doubleKills"`
	DragonKills                    int    `json:"dragonKills"`
	FirstBloodAssist               bool   `json:"firstBloodAssist"`
	FirstBloodKill                 bool   `json:"firstBloodKill"`
	FirstTowerAssist               bool   `json:"firstTowerAssist"`
	FirstTowerKill                 bool   `json:"firstTowerKill"`
	GameEndedInEarlySurrender      bool   `json:"gameEndedInEarlySurrender"`
	GameEndedInSurrender           bool   `json:"gameEndedInSurrender"`
	GoldEarned                     int    `json:"goldEarned"`
	GoldSpent                      int    `json:"goldSpent"`
	IndividualPosition             string `json:"individualPosition"`
	InhibitorKills                 int    `json:"inhibitorKills"`
	InhibitorTakedowns             int    `json:"inhibitorTakedowns"`
	InhibitorsLost                 int    `json:"inhibitorsLost"`
	Item0                          int    `json:"item0"`
	Item1                          int    `json:"item1"`
	Item2                          int    `json:"item2"`
	Item3                          int    `json:"item3"`
	Item4                          int    `json:"item4"`
	Item5                          int    `json:"item5"`
	Item6                          int    `json:"item6"`
	ItemsPurchased                 int    `json:"itemsPurchased"`
	KillingSprees                  int    `json:"killingSprees"`
	Kills                          int    `json:"kills"`
	Lane                           string `json:"lane"`
	LargestCriticalStrike          int    `json:"largestCriticalStrike"`
	LargestKillingSpree            int    `json:"largestKillingSpree"`
	LargestMultiKill               int    `json:"largestMultiKill"`
	LongestTimeSpentLiving         int    `json:"longestTimeSpentLiving"`
	MagicDamageDealt               int    `json:"magicDamageDealt"`
	MagicDamageDealtToChampions    int    `json:"magicDamageDealtToChampions"`
	MagicDamageTaken               int    `json:"magicDamageTaken"`
	NeutralMinionsKilled           int    `json:"neutralMinionsKilled"`
	NexusKills                     int    `json:"nexusKills"`
	NexusTakedowns                 int    `json:"nexusTakedowns"`
	NexusLost                      int    `json:"nexusLost"`
	ObjectivesStolen               int    `json:"objectivesStolen"`
	ObjectivesStolenAssists        int    `json:"objectivesStolenAssists"`
	ParticipantId                  int    `json:"participantId"`
	PentaKills                     int    `json:"pentaKills"`
	PhysicalDamageDealt            int    `json:"physicalDamageDealt"`
	PhysicalDamageDealtToChampions int    `json:"physicalDamageDealtToChampions"`
	PhysicalDamageTaken            int    `json:"physicalDamageTaken"`
	ProfileIcon                    int    `json:"profileIcon"`
	Puuid                          string `json:"puuid"`
	QuadraKills                    int    `json:"quadraKills"`
	RiotIdName                     string `json:"riotIdName"`
	RiotIdTagline                  string `json:"riotIdTagline"`
	Role                           string `json:"role"`
	SightWardsBoughtInGame         int    `json:"sightWardsBoughtInGame"`
	Spell1Casts                    int    `json:"spell1Casts"`
	Spell2Casts                    int    `json:"spell2Casts"`
	Spell3Casts                    int    `json:"spell3Casts"`
	Spell4Casts                    int    `json:"spell4Casts"`
	Summoner1Casts                 int    `json:"summoner1Casts"`
	Summoner1Id                    int    `json:"summoner1Id"`
	Summoner2Casts                 int    `json:"summoner2Casts"`
	Summoner2Id                    int    `json:"summoner2Id"`
	SummonerId                     string `json:"summonerId"`
	SummonerLevel                  int    `json:"summonerLevel"`
	SummonerName                   string `json:"summonerName"`
	TeamEarlySurrendered           bool   `json:"teamEarlySurrendered"`
	TeamId                         int    `json:"teamId"`
	TeamPosition                   string `json:"teamPosition"`
	TimeCCingOthers                int    `json:"timeCCingOthers"`
	TimePlayed                     int    `json:"timePlayed"`
	TotalDamageDealt               int    `json:"totalDamageDealt"`
	TotalDamageDealtToChampions    int    `json:"totalDamageDealtToChampions"`
	TotalDamageShieldedOnTeammates int    `json:"totalDamageShieldedOnTeammates"`
	TotalDamageTaken               int    `json:"totalDamageTaken"`
	TotalHeal                      int    `json:"totalHeal"`
	TotalHealsOnTeammates          int    `json:"totalHealsOnTeammates"`
	TotalMinionsKilled             int    `json:"totalMinionsKilled"`
	TotalTimeCCDealt               int    `json:"totalTimeCCDealt"`
	TotalTimeSpentDead             int    `json:"totalTimeSpentDead"`
	TotalUnitsHealed               int    `json:"totalUnitsHealed"`
	TripleKills                    int    `json:"tripleKills"`
	TrueDamageDealt                int    `json:"trueDamageDealt"`
	TrueDamageDealtToChampions     int    `json:"trueDamageDealtToChampions"`
	TrueDamageTaken                int    `json:"trueDamageTaken"`
	TurretKills                    int    `json:"turretKills"`
	TurretTakedowns                int    `json:"turretTakedowns"`
	TurretsLost                    int    `json:"turretsLost"`
	UnrealKills                    int    `json:"unrealKills"`
	VisionWardsBoughtInGame        int    `json:"visionWardsBoughtInGame"`
	WardsKilled                    int    `json:"wardsKilled"`
	WardsPlaced                    int    `json:"wardsPlaced"`
	Win                            bool   `json:"win"`
	TotalDamageDealtToObjectives   int    `json:"totalDamageDealtToObjectives"`
	Tier                           string `json:"tier"`
	Rank                           string `json:"rank"`
	LeaguePoints                   int    `json:"leaguePoints"`
	Wins                           int    `json:"wins"`
	Losses                         int    `json:"losses"`
}

// ExtractParticipantDataAtIndex extracts the data of a participant at a specific index from matchData
func ExtractParticipantDataAtIndex(matchData map[string]interface{}, apiKey, region string, index int) (*ParticipantData, error) {
	// Access the info key to get the participants data
	info, exists := matchData["info"].(map[string]interface{})
	if !exists {
		return nil, errors.New("'info' key not found in matchData")
	}

	participants, ok := info["participants"].([]interface{})
	if !ok || index < 0 || index >= len(participants) {
		return nil, errors.New("Invalid participants data or index")
	}

	participantMap, ok := participants[index].(map[string]interface{})
	if !ok {
		return nil, errors.New("Participant data at specified index is not of expected type")
	}

	data := &ParticipantData{}
	err := mapstructure.Decode(participantMap, data)
	if err != nil {
		return nil, err
	}

	// Ensure summonerId exists before fetching league data
	if data.SummonerId == "" {
		return nil, errors.New("failed to retrieve summoner ID for participant")
	}

	leagueData, err := fetchLeagueDataBySummonerID(data.SummonerId, apiKey, region)
	if err != nil {
		log.Println("Error fetching league data:", err)
	} else {
		data.Tier = safeString(leagueData, "tier")
		data.Rank = safeString(leagueData, "rank")
		data.LeaguePoints = safeInt(leagueData, "leaguePoints")
		data.Wins = safeInt(leagueData, "wins")
		data.Losses = safeInt(leagueData, "losses")
	}

	return data, nil
}

func safeString(data map[string]interface{}, key string) string {
	if value, ok := data[key]; ok {
		return value.(string)
	}
	return ""
}

func safeInt(data map[string]interface{}, key string) int {
	if value, ok := data[key]; ok && value != nil {
		return int(value.(float64))
	}
	return 0
}

func safeBool(data map[string]interface{}, key string) bool {
	if value, ok := data[key].(bool); ok {
		return value
	}
	return false
}

func fetchLeagueDataBySummonerID(summonerID string, apiKey string, region string) (map[string]interface{}, error) {
	// Check cache first
	cacheMutex.RLock()
	cachedItem, exists := leagueDataCache[summonerID]
	cacheMutex.RUnlock()
	if exists && time.Since(cachedItem.Timestamp) < cacheExpiryDuration {
		return cachedItem.Data, nil
	}

	riotBaseUrl, exists := regionMapping[region]
	if !exists {
		return nil, fmt.Errorf("Invalid region: %s", region)
	}

	url := fmt.Sprintf("https://%s/lol/league/v4/entries/by-summoner/%s", riotBaseUrl, summonerID)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Riot-Token", apiKey)

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var leagueData []map[string]interface{}
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&leagueData); err != nil {
		return nil, err
	}

	for _, entry := range leagueData {
		if queue, ok := entry["queueType"].(string); ok && queue == "RANKED_SOLO_5x5" {
			// Save to cache
			cacheMutex.Lock()
			leagueDataCache[summonerID] = CacheItem{Data: entry, Timestamp: time.Now()}
			cacheMutex.Unlock()

			return entry, nil
		}
	}

	return nil, errors.New("No RANKED_SOLO_5x5 data found for the given summoner ID")
}

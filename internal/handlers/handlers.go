package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/drewfoos/league-finder/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/patrickmn/go-cache"
	"github.com/valyala/fasthttp"
)

type CacheItem struct {
	Data      map[string]interface{}
	Timestamp time.Time
}

var apiKey string

// Define a global cache with a mutex for thread-safety
var leagueDataCache = cache.New(24*time.Hour, 30*time.Minute)
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
	Region string `json:"region"`
	Count  int    `json:"count"`
	Start  int    `json:"start"`
}

type SummonerData struct {
	PUUID string `json:"puuid"`
}

func writeErrorResponse(w http.ResponseWriter, logMsg string, httpMsg string, code int) {
	log.Println(logMsg)
	http.Error(w, httpMsg, code)
}

func createRequest(method string, url string, apiKey string) *fasthttp.Request {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	req.Header.Set("X-Riot-Token", apiKey)
	return req
}

var summonerDataCache = cache.New(24*time.Hour, 30*time.Minute)

func getSummonerData(endpoint, apiKey string) (*SummonerData, error) {
	if x, found := summonerDataCache.Get(endpoint); found {
		return x.(*SummonerData), nil
	}

	req := createRequest("GET", endpoint, apiKey)
	defer fasthttp.ReleaseRequest(req)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		return nil, fmt.Errorf("Riot API returned non-200 status code %d", statusCode)
	}

	var summoner SummonerData
	err = json.Unmarshal(resp.Body(), &summoner)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	summonerDataCache.Set(endpoint, &summoner, cache.DefaultExpiration)

	return &summoner, nil
}

func InitApiKey() {
	apiKey = strings.TrimSpace(utils.GetApiKeyFromFile(".environment.env"))
	if apiKey == "" {
		log.Fatal("API key not found in .environment.env")
	}
}

func SearchHandler(c *fiber.Ctx) error {
	body := c.Body()
	var request UrlRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	riotBaseUrl, exists := regionMapping[request.Region]
	if !exists {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid region"})
	}

	count := request.Count
	start := request.Start

	summoner, err := getSummonerData(fmt.Sprintf("https://%s%s", riotBaseUrl, strings.TrimSpace(request.Url)), apiKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch summoner data"})
	}

	matchIDs, err := fetchMatchesByPUUID(summoner.PUUID, apiKey, request.Region, start, count)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch matches"})
	}

	if len(matchIDs) == 0 {
		return c.JSON(summoner)
	}

	participantDataList, err := fetchAndProcessMatchDetailsForIDs(matchIDs, apiKey, summoner.PUUID, request.Region)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch match details"})
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

const baseURLFormatForMatches = "https://%s/lol/match/v5/matches/by-puuid/%s/ids?start=%d&count=%d"

func fetchMatchesByPUUID(puuid string, apiKey string, region string, start int, count int) ([]string, error) {
	url := fmt.Sprintf(baseURLFormatForMatches, getPlatformForRegion(region), strings.TrimSpace(puuid), start, count)

	req := createRequest("GET", url, apiKey)
	defer fasthttp.ReleaseRequest(req)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := fasthttp.Do(req, resp)
	if err != nil {
		log.Printf("Error fetching matches by PUUID %s: %v", puuid, err)
		return nil, err
	}

	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		errMsg := fmt.Sprintf("Riot API returned non-200 status code %d while fetching matches by PUUID %s", statusCode, puuid)
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	var matchIDs []string
	if err := json.Unmarshal(resp.Body(), &matchIDs); err != nil {
		log.Printf("Error decoding JSON response from Riot API for PUUID %s: %v", puuid, err)
		return nil, err
	}

	return matchIDs, nil
}

const baseURLFormat = "https://%s/lol/match/v5/matches/%s"

func fetchMatchDetails(matchID string, apiKey string, region string) (map[string]interface{}, error) {
	url := fmt.Sprintf(baseURLFormat, getPlatformForRegion(region), strings.TrimSpace(matchID))

	req := createRequest("GET", url, apiKey)
	defer fasthttp.ReleaseRequest(req)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := fasthttp.Do(req, resp)
	if err != nil {
		log.Printf("Error fetching match details for match ID %s: %v", matchID, err)
		return nil, err
	}

	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		errMsg := fmt.Sprintf("Riot API returned non-200 status code %d while fetching match details for match ID %s", statusCode, matchID)
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	var matchData map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &matchData); err != nil {
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
	dataList.Data = make([]ParticipantData, 0)

	var wg sync.WaitGroup
	var once sync.Once
	errCh := make(chan string, len(matchIDs))
	errChMutex := &sync.Mutex{}

	sem := make(chan struct{}, 10)

	for _, matchID := range matchIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			matchData, err := fetchMatchDetails(id, apiKey, region)
			if err != nil {
				errChMutex.Lock()
				errCh <- "Error fetching details for match ID: " + id + " - " + err.Error()
				errChMutex.Unlock()
				return
			}

			metadata, exists := matchData["metadata"].(map[string]interface{})
			if !exists {
				errChMutex.Lock()
				errCh <- "metadata doesn't exist for match ID: " + id
				errChMutex.Unlock()
				return
			}

			participants, exists := metadata["participants"].([]interface{})
			if !exists {
				errChMutex.Lock()
				errCh <- "No participants array in metadata for match ID: " + id
				errChMutex.Unlock()
				return
			}

			var mainParticipantIndex int
			for index, participant := range participants {
				participantPUUID, ok := participant.(string)
				if ok && participantPUUID == puuid {
					mainParticipantIndex = index
					break
				}
			}

			participantDataList, err := ExtractParticipantData(matchData, metadata, apiKey, region, mainParticipantIndex)
			if err != nil {
				errChMutex.Lock()
				errCh <- "Error extracting participant data for match ID: " + id + " - " + err.Error()
				errChMutex.Unlock()
				return
			}

			cacheMutex.Lock()
			dataList.Data = append(dataList.Data, participantDataList...)
			cacheMutex.Unlock()
		}(matchID)
	}

	go func() {
		wg.Wait()
		once.Do(func() { close(errCh) })
	}()

	for err := range errCh {
		log.Println(err)
	}

	return dataList, nil
}

type ParticipantData struct {
	MatchId                        string `json:"matchId"`
	IsMainParticipant              bool   `json:"isMainParticipant"`
	QueueDescription               string `json:"queueDescription"`
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

var queueIdMap = map[int]string{
	0:    "Custom",
	2:    "Blind",
	4:    "Ranked",
	6:    "Ranked",
	7:    "Co-op vs AI",
	8:    "3v3 Normal games",
	9:    "3v3 Ranked Flex games",
	14:   "5v5 Draft Pick games",
	16:   "5v5 Dominion Blind Pick games",
	17:   "5v5 Dominion Draft Pick games",
	25:   "Dominion Co-op vs AI games",
	31:   "Co-op vs AI Intro Bot",
	32:   "Co-op vs AI Beginner Bot",
	33:   "Co-op vs AI Intermediate Bot",
	41:   "3v3 Ranked Team games",
	42:   "Ranked",
	52:   "Twisted Treeline Co-op vs AI games",
	61:   "5v5 Team Builder games",
	65:   "ARAM",
	67:   "ARAM Co-op vs AI games",
	70:   "One for All games",
	72:   "1v1 Snowdown Showdown games",
	73:   "2v2 Snowdown Showdown games",
	75:   "6v6 Hexakill games",
	76:   "Ultra Rapid Fire games",
	78:   "One For All: Mirror Mode games",
	83:   "Co-op vs AI Ultra Rapid Fire games",
	91:   "Doom Bots Rank 1 games",
	92:   "Doom Bots Rank 2 games",
	93:   "Doom Bots Rank 5 games",
	96:   "Ascension games",
	98:   "Twisted Treeline 6v6 Hexakill games",
	100:  "Butcher's Bridge 5v5 ARAM games",
	300:  "Legend of the Poro King games",
	310:  "Nemesis games",
	313:  "Black Market Brawlers games",
	315:  "Nexus Siege games",
	317:  "Definitely Not Dominion games",
	318:  "ARURF games",
	325:  "All Random games",
	400:  "Draft",
	410:  "Ranked",
	420:  "Ranked",
	430:  "5v5 Blind Pick games",
	440:  "Flex",
	450:  "ARAM",
	460:  "3v3 Blind Pick games",
	470:  "3v3 Ranked Flex games",
	600:  "Blood Hunt Assassin games",
	610:  "Dark Star: Singularity games",
	700:  "Summoner's Rift Clash games",
	720:  "ARAM Clash",
	800:  "Twisted Treeline Co-op vs. AI Intermediate Bot games",
	810:  "Twisted Treeline Co-op vs. AI Intro Bot games",
	820:  "Twisted Treeline Co-op vs. AI Beginner Bot games",
	830:  "Co-op vs. AI Intro Bot games",
	840:  "Rift Co-op vs. AI Beginner Bot games",
	850:  "Co-op vs. AI Intermediate Bot games",
	900:  "ARURF games",
	910:  "Ascension games",
	920:  "Legend of the Poro King games",
	940:  "Nexus Siege games",
	950:  "Doom Bots Voting games",
	960:  "Doom Bots Standard games",
	980:  "Star Guardian Invasion: Normal games",
	990:  "Star Guardian Invasion: Onslaught games",
	1000: "PROJECT: Hunters games",
	1010: "Snow ARURF games",
	1020: "One for All games",
	1030: "Odyssey Extraction: Intro games",
	1040: "Odyssey Extraction: Cadet games",
	1050: "Odyssey Extraction: Crewmember games",
	1060: "Odyssey Extraction: Captain games",
	1070: "Odyssey Extraction: Onslaught games",
	1090: "Teamfight Tactics games",
	1100: "Ranked Teamfight Tactics games",
	1110: "Teamfight Tactics Tutorial games",
	1111: "Teamfight Tactics test games",
	1200: "Nexus Blitz games",
	1300: "Nexus Blitz games",
	1400: "Ultimate Spellbook games",
	1700: "Arena",
	1900: "Pick URF games",
	2000: "Tutorial 1",
	2010: "Tutorial 2",
	2020: "Tutorial 3",
}

func ExtractParticipantData(matchData map[string]interface{}, metadata map[string]interface{}, apiKey, region string, mainParticipantIndex int) ([]ParticipantData, error) {
	info, exists := matchData["info"].(map[string]interface{})
	if !exists {
		return nil, errors.New("'info' key not found in matchData")
	}

	queueId := safeInt(info, "queueId")
	queueDescription, exists := queueIdMap[queueId]
	if !exists {
		queueDescription = "Unknown Game Type"
	}

	participants, ok := info["participants"].([]interface{})
	if !ok {
		return nil, errors.New("Invalid participants data")
	}

	if mainParticipantIndex < 0 || mainParticipantIndex >= len(participants) {
		return nil, errors.New("Invalid main participant index")
	}

	var dataList []ParticipantData

	matchId := safeString(metadata, "matchId")

	for i, participant := range participants {
		participantMap, ok := participant.(map[string]interface{})
		if !ok {
			return nil, errors.New("Participant data at specified index is not of expected type")
		}

		data := &ParticipantData{}
		err := mapstructure.Decode(participantMap, data)
		if err != nil {
			return nil, err
		}

		data.QueueDescription = queueDescription

		if i == mainParticipantIndex {
			// Ensure summonerId exists before fetching league data
			if data.SummonerId == "" {
				return nil, errors.New("failed to retrieve summoner ID for participant")
			}

			leagueData, err := fetchLeagueDataBySummonerID(data.SummonerId, apiKey, region)
			if err != nil {
				log.Println("Error fetching league data:", err)
			} else {
				tier := strings.ToLower(safeString(leagueData, "tier"))
				tier = strings.ToUpper(string(tier[0])) + tier[1:]
				data.Tier = tier
				data.Rank = safeString(leagueData, "rank")
				data.LeaguePoints = safeInt(leagueData, "leaguePoints")
				data.Wins = safeInt(leagueData, "wins")
				data.Losses = safeInt(leagueData, "losses")
			}

			data.IsMainParticipant = true
		} else {
			data.IsMainParticipant = false
		}

		data.MatchId = matchId

		dataList = append(dataList, *data)
	}

	return dataList, nil
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
	if x, found := leagueDataCache.Get(summonerID); found {
		return x.(map[string]interface{}), nil
	}

	riotBaseUrl, exists := regionMapping[region]
	if !exists {
		return nil, fmt.Errorf("Invalid region: %s", region)
	}

	url := fmt.Sprintf("https://%s/lol/league/v4/entries/by-summoner/%s", riotBaseUrl, summonerID)
	req := createRequest("GET", url, apiKey)
	defer fasthttp.ReleaseRequest(req)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		return nil, fmt.Errorf("Riot API returned non-200 status code %d", statusCode)
	}

	var leagueData []map[string]interface{}
	err = json.Unmarshal(resp.Body(), &leagueData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	for _, entry := range leagueData {
		if queue, ok := entry["queueType"].(string); ok && queue == "RANKED_SOLO_5x5" {
			// Save to cache
			leagueDataCache.Set(summonerID, entry, cache.DefaultExpiration)

			return entry, nil
		}
	}

	return nil, errors.New("No RANKED_SOLO_5x5 data found for the given summoner ID")
}

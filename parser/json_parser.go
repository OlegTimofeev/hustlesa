package parser

import (
	"bitbucket.org/Axxonsoft/axxoncloudgo/util"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/itimofeev/hustlesa/model"
	"gopkg.in/mgutz/dat.v1"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var digitPattern = regexp.MustCompile("\\d+")
var letterPattern = regexp.MustCompile("[ABCDERMS]")

func Parse(dirName string) model.RawParsingResults {
	name2club := make(map[string]int64)

	clubs := make([]model.RawClub, 0)
	dancers := make([]model.RawDancer, 0)
	dancerClubs := make([]model.RawDancerClub, 0)
	competitions := make([]model.RawCompetition, 0)
	jnjCompetitions := make([]model.RawCompetition, 0)
	nominations := make([]model.RawNomination, 0)
	jnjNominations := make([]model.RawNomination, 0)
	compResults := make([]model.RawCompetitionResult, 0)
	jnjResults := make([]model.RawCompetitionResult, 0)
	loadFromJSON(dirName+"clubs.json", &clubs)
	loadFromJSON(dirName+"dancers.json", &dancers)
	loadFromJSON(dirName+"dancerClubs.json", &dancerClubs)
	loadFromJSON(dirName+"competitions.json", &competitions)
	loadFromJSON(dirName+"nominations.json", &nominations)
	loadFromJSON(dirName+"competitionsResults.json", &compResults)
	loadFromJSON(dirName+"jnjCompetitions.json", jnjCompetitions)
	loadFromJSON(dirName+"jnjNominations.json", jnjNominations)
	loadFromJSON(dirName+"jnjResults.json", jnjResults)

	clubs = fixClubs(clubs)
	dancers = fixDancers(dancers)
	fillName2Club(name2club, clubs)
	dancerClubs = fixDancerClubs(dancerClubs, name2club)
	competitions = fixCompetitions(competitions)
	nominations = fixNominations(nominations)
	compResults = fixCompResults(compResults)

	site2oldCompId := make(map[string]int64)
	for _, comp := range competitions {
		site2oldCompId[comp.Site.String] = comp.ID
	}

	new2old := make(map[int64]int64)

	jnjNominations = fixJnjNominations(jnjNominations)
	jnjCompetitions = fixJnjCompetitionIds(jnjCompetitions, site2oldCompId, new2old)
	jnjNominations = fixJnjNominationCompetitionIds(jnjNominations, new2old)

	jnjResults = fixJnjResults(jnjResults)

	return model.RawParsingResults{
		Clubs:        clubs,
		Dancers:      dancers,
		DancerClubs:  dancerClubs,
		Competitions: competitions,
		Nominations:  nominations,
		CompResults:  compResults,

		JnjCompetitions: jnjCompetitions,
		JnjNominations:  jnjNominations,
		JnjCompResults:  jnjResults,
	}
}

func fixJnjResults(results []model.RawCompetitionResult) []model.RawCompetitionResult {
	for i := range results {
		fixJnjResult(&results[i])
	}
	return results
}

func fixJnjResult(result *model.RawCompetitionResult) *model.RawCompetitionResult {
	s := result.Result
	s = doCleanJnj(s)
	split := strings.Split(s, ")")

	var placesStr, allPlacesStr string

	if len(split) > 1 {
		allPlacesStr = strings.Replace(split[0], ")", "", -1)
		placesStr = split[1]
	} else {
		placesStr = split[0]
	}

	className := placesStr[:1]
	placesSplit := strings.Split(placesStr[1:], "/")
	placeSplitOnPlus := strings.Split(placesSplit[1], "+")

	points := 0
	if len(placeSplitOnPlus) == 2 {
		points = Atoi(placeSplitOnPlus[1])
	}

	place := Atoi(placesSplit[0])
	placeFrom := Atoi(placeSplitOnPlus[0])

	result.Place = place
	result.PlaceFrom = placeFrom
	result.IsJNJ = true
	result.Class = uncompressJnjClass(className)
	result.Points = points

	if allPlacesStr == "" {
		result.AllPlacesMinClass = result.Class
		result.AllPlacesMaxClass = result.Class
		result.AllPlacesFrom = result.Place
		result.AllPlacesTo = result.Place
	} else {
		cleanAllPlaceStr := allPlacesStr
		minClass, maxClass := parseClasses(cleanAllPlaceStr, false)
		numbers := parseAllNumbers(cleanAllPlaceStr)

		allPlaceFrom, allPlaceTo := 0, 0
		if len(numbers) == 2 {
			allPlaceFrom = numbers[0]
			allPlaceTo = numbers[1]
		} else if len(numbers) == 1 {
			allPlaceFrom = numbers[0]
			allPlaceTo = numbers[0]
		} else {
			CheckOk(false, "Bad format "+allPlacesStr)
		}

		result.AllPlacesMinClass = uncompressJnjClass(minClass)
		result.AllPlacesMaxClass = uncompressJnjClass(maxClass)
		result.AllPlacesFrom = allPlaceFrom
		result.AllPlacesTo = allPlaceTo
	}

	return result
}
func uncompressJnjClass(class string) string {
	switch class {
	case "R":
		return "RS"
	case "C":
		return "Ch"
	case "M":
		return "M"
	case "B":
		return "BG"
	case "S":
		return "S"
	}
	CheckOk(false, "unknown class "+class)
	return class
}

func fixJnjNominationCompetitionIds(nominations []model.RawNomination, new2old map[int64]int64) []model.RawNomination {
	for i := range nominations {
		newId := nominations[i].CompetitionID

		oldId, ok := new2old[newId]
		CheckOk(ok, fmt.Sprintf("not found comp by id %d", newId))

		nominations[i].CompetitionID = oldId
	}
	return nominations
}

func fixJnjNominations(nominations []model.RawNomination) []model.RawNomination {
	for i := range nominations {
		fixJnjNomination(&nominations[i])
	}

	return nominations
}

func fixJnjNomination(nomination *model.RawNomination) *model.RawNomination {
	s := nomination.Value
	s = doCleanJnj(s)

	minClass, maxClass := parseClasses(s, false)
	maleCount, femaleCount := parse2Numbers(s)

	nomination.Type = "NEW_JNJ"
	nomination.MaleCount = maleCount
	nomination.FemaleCount = femaleCount
	nomination.MinJnjClass = dat.NullStringFrom(minClass)
	nomination.MaxJnjClass = dat.NullStringFrom(maxClass)

	return nomination
}

func doCleanJnj(s string) string {
	s = strings.Replace(s, " ", "", -1)
	s = strings.Replace(s, "уч.", "", -1)
	s = strings.Replace(s, "B-RS1", "BG-RS1", -1)
	s = strings.Replace(s, "Ch10/11", "S10/11", -1)
	s = strings.Replace(s, "B33/34", "BG33/34", -1)
	s = strings.Replace(s, "BGG", "B", -1)
	s = strings.Replace(s, "BG", "B", -1)
	s = strings.Replace(s, "Bg", "B", -1)
	s = strings.Replace(s, "RS", "R", -1)
	s = strings.Replace(s, "Rs", "R", -1)
	s = strings.Replace(s, "Ch", "C", -1)
	s = strings.Replace(s, "CH", "C", -1)
	return s
}

func fixJnjCompetitionIds(jnj []model.RawCompetition, site2oldCompId map[string]int64, new2old map[int64]int64) []model.RawCompetition {
	for i := range jnj {
		CheckOk(jnj[i].Site.Valid, "jnj site is empty")

		oldId, ok := site2oldCompId[jnj[i].Site.String]
		CheckOk(ok, "old id not found by site jnj[i].Site")

		new2old[jnj[i].ID] = oldId

		jnj[i].ID = oldId
	}

	return jnj
}

func fixCompResults(results []model.RawCompetitionResult) []model.RawCompetitionResult {
	newResults := make([]model.RawCompetitionResult, 0, len(results))
	for i := range results {
		fixed := fixResult(&results[i])
		if fixed != nil {
			newResults = append(newResults, *fixed)
		}
	}

	return newResults
}

func fixResult(result *model.RawCompetitionResult) *model.RawCompetitionResult {
	//(Д-А30-32)C1/19+5
	//(С-Б6)C4/9+1
	//E4/21+3

	s := result.Result
	s = doCleanResult(s)
	s = doCleanCompDependent(s, result.CompetitionID)

	if strings.Contains(strings.ToLower(s), "x") || strings.Contains(s, "skip") || strings.Contains(s, "анулировано") || strings.Contains(s, "штраф") {
		//TODO process x
		return nil
	}
	split := strings.Split(s, ")")
	var allPlacesStr, placesStr string

	if len(split) > 1 {
		allPlacesStr = strings.Replace(split[0], "(", "", -1)
		placesStr = split[1]
	} else {
		placesStr = s
	}

	placesStr = strings.Replace(placesStr, "/0", "", -1) //spike
	className := placesStr[:1]
	placesSplit := strings.Split(placesStr[1:], "/")
	placeSplitOnPlus := strings.Split(placesSplit[1], "+")

	points := 0
	if len(placeSplitOnPlus) == 2 {
		points = Atoi(placeSplitOnPlus[1])
	}

	place := Atoi(placesSplit[0])
	placeFrom := Atoi(placeSplitOnPlus[0])
	isJnj := strings.Contains(allPlacesStr, "@")

	result.Place = place
	result.PlaceFrom = placeFrom
	result.IsJNJ = isJnj
	result.Class = className
	result.Points = points

	if allPlacesStr == "" {
		result.AllPlacesMinClass = result.Class
		result.AllPlacesMaxClass = result.Class
		result.AllPlacesFrom = result.Place
		result.AllPlacesTo = result.Place
	} else {
		cleanAllPlaceStr := strings.Replace(allPlacesStr, "@", "", -1) //D-E12-13 D-E12 E12-13 CBA12
		minClass, maxClass := parseClasses(cleanAllPlaceStr, true)
		numbers := parseAllNumbers(cleanAllPlaceStr)

		allPlaceFrom, allPlaceTo := 0, 0
		if len(numbers) == 2 {
			allPlaceFrom = numbers[0]
			allPlaceTo = numbers[1]
		} else if len(numbers) == 1 {
			allPlaceFrom = numbers[0]
			allPlaceTo = numbers[0]
		} else {
			CheckOk(false, "Bad format "+allPlacesStr)
		}

		result.AllPlacesMinClass = minClass
		result.AllPlacesMaxClass = maxClass
		result.AllPlacesFrom = allPlaceFrom
		result.AllPlacesTo = allPlaceTo
	}

	return result
}
func fixNominations(nominations []model.RawNomination) []model.RawNomination {

	newNominations := make([]model.RawNomination, 0, len(nominations))
	for i := range nominations {
		nomination := fixNomination(&nominations[i])
		if nomination != nil {
			newNominations = append(newNominations, *nomination)
		}
	}

	E := "E"
	C := "C"
	D := "D"
	B := "B"

	newNominations = append(newNominations, []model.RawNomination{
		{
			Type:          "CLASSIC",
			CompetitionID: 247,
			FemaleCount:   2,
			MaleCount:     2,
			MinClass:      dat.NullStringFrom(E),
			MaxClass:      dat.NullStringFrom(C),
		},
		{ //"Чемпионат Москвы 2014"
			Type:          "CLASSIC",
			CompetitionID: 238,
			FemaleCount:   57,
			MaleCount:     57,
			MinClass:      dat.NullStringFrom(D),
			MaxClass:      dat.NullStringFrom(D),
		},
		{ //Кубок Буревестника 2013
			Type:          "CLASSIC",
			CompetitionID: 213,
			FemaleCount:   3,
			MaleCount:     3,
			MinClass:      dat.NullStringFrom(B),
			MaxClass:      dat.NullStringFrom(B),
		},
		{ //Кубок В.Новгорода 2011
			Type:          "CLASSIC",
			CompetitionID: 109,
			FemaleCount:   10,
			MaleCount:     10,
			MinClass:      dat.NullStringFrom(C),
			MaxClass:      dat.NullStringFrom(B),
		},
	}...)

	return newNominations
}

func fixNomination(nomination *model.RawNomination) *model.RawNomination {
	if strings.Contains(nomination.Value, "снят рейт.") {
		return nil
	}
	s := doCleanNomination(nomination.Value)
	s = doCleanCompDependent(s, nomination.CompetitionID)

	if strings.Contains(s, "skip") {
		return nil
	}

	isJnj := strings.Contains(s, "@")
	if isJnj {
		nomination.Type = "OLD_JNJ"
		mCount, fCount := parse2Numbers(s)
		nomination.MaleCount = mCount
		nomination.FemaleCount = fCount
	} else {
		nomination.Type = "CLASSIC"
		count := parseNumber(s)
		nomination.MaleCount = count
		nomination.FemaleCount = count
	}
	s = strings.Replace(s, "@", "", -1)

	minClass, maxClass := parseClasses(s, true)

	nomination.MinClass = dat.NullStringFrom(minClass)
	nomination.MaxClass = dat.NullStringFrom(maxClass)

	return nomination
}

func parseAllNumbers(str string) []int {
	numbers := digitPattern.FindAllString(str, -1)

	res := make([]int, 0, len(numbers))

	for _, number := range numbers {
		n, err := strconv.Atoi(number)
		CheckErr(err, "unable to parse int "+number)
		res = append(res, n)
	}
	return res
}

func parseNumber(str string) int {
	numbers := parseAllNumbers(str)
	CheckOk(len(numbers) == 1, fmt.Sprintf("Len of numbers not 1: %v, %s", numbers, str))

	return numbers[0]
}

func parse2Numbers(str string) (int, int) {
	numbers := parseAllNumbers(str)
	CheckOk(len(numbers) == 2, fmt.Sprintf("Len of numbers not 2: %v, %s", numbers, str))

	return numbers[0], numbers[1]
}

func parseAllLetters(str string) []string {
	return letterPattern.FindAllString(str, -1)
}

func parseClasses(s string, classic bool) (string, string) {
	letters := parseAllLetters(s)
	var m map[string]int
	if classic {
		m = map[string]int{"A": 10, "B": 8, "C": 6, "D": 4, "E": 2}
	} else {
		m = map[string]int{"C": 10, "S": 8, "M": 6, "R": 4, "B": 2}
	}

	minClass := letters[0]
	maxClass := letters[0]

	for _, let := range letters {
		if m[let] < m[minClass] {
			minClass = let
		}
		if m[let] > m[maxClass] {
			maxClass = let
		}
	}

	return minClass, maxClass
}

func doCleanNomination(s string) string {
	s = doCleanResult(s)
	s = strings.Replace(s, "+", "", -1)
	return s
}

func doCleanResult(s string) string {
	s = strings.Replace(s, "ДнД", "@", -1)
	s = strings.Replace(s, "E/D 63/57 пар", "E63", -1)
	s = strings.Replace(s, "Абсолют", "ABC", -1)
	s = strings.Replace(s, "А", "A", -1)
	s = strings.Replace(s, "В", "B", -1)
	s = strings.Replace(s, "Б", "B", -1)
	s = strings.Replace(s, "С", "C", -1)
	s = strings.Replace(s, "Д", "D", -1)
	s = strings.Replace(s, "Е", "E", -1)
	s = strings.Replace(s, "Х", "X", -1)
	s = strings.Replace(s, "\"", "", -1)
	s = strings.Replace(s, "пары", "", -1)
	s = strings.Replace(s, "пара", "", -1)
	s = strings.Replace(s, "пар", "", -1)
	s = strings.Replace(s, "уч.", "", -1)
	s = strings.Replace(s, "уч", "", -1)
	s = strings.Replace(s, " ", "", -1)
	//.replaceAll("\\s+", "")

	return s
}

func doCleanCompDependent(s string, competitionID int64) string {
	switch competitionID {
	case 214:
		s = strings.Replace(s, "@DCBA", "@EDCBA", -1)
	case 221:
		s = strings.Replace(s, "@CBA", "@BA", -1)
	case 267:
		s = strings.Replace(s, "D-A56", "E-A56", -1)
	case 259:
		if "C3" == s {
			s = "skip"
		}
	case 230:
		if "CBA6" == s {
			s = "EA6" //Nord Cup г.СПб
		}
	case 228:
		if "BA6" == s {
			s = "EA6" //Кубок АСХ г.Москва
		}
	case 223:
		if "CBA4" == s {
			s = "EA4" //Кубок Движения г.Москва
		}
	case 238:
		if "E/D63/57" == s {
			s = "E63" //Чемпионат Москвы 2014
		}
	case 213:
		if "C13/B3" == s {
			s = "C13" //Кубок Буревестника 2013
		}
	case 142:
		if "DCBA9" == s {
			s = "EA9" //Кубок Буревестника 2013
		}
	case 135:
		if "CBAA83" == s {
			s = "DA83" //Чемпионат России 2011
		}
		if "C79" == s {
			s = "E79" //Чемпионат России 2011
		}
	case 132:
		if "DCBA23" == s {
			s = "EA23" //Буревестник 2011
		}
	case 125:
		if "B+A3" == s {
			s = "BA3" //Буревестник 2011
		}
	case 91:
		if "CBA16" == s {
			s = "EA16" //Буревестник 2011
		}
	case 81:
		if "CBA12" == s {
			s = "DA12" //Буревестник 2011
		}
	}

	return s
}

func fixCompetitions(competitions []model.RawCompetition) []model.RawCompetition {
	for i, c := range competitions {
		competitions[i].Date = parseFromUnix(c.RawDate)
	}
	return competitions
}
func parseFromUnix(timeInUnix int64) time.Time {
	return time.Unix(timeInUnix/1000, 0) //TODO разобраться, какая-то хрень
}

func parseDancerName(name string) (string, string, *string) {
	split := strings.Split(name, " ")
	if !(len(split) == 2 || len(split) == 3) {
		log.Panic("Bad name " + name)
	}

	if len(split) == 2 {
		return split[0], split[1], nil
	}
	return split[0], split[1], &split[2]
}

func fixDancers(dancers []model.RawDancer) []model.RawDancer {
	for i, dancer := range dancers {
		dancers[i].Code = fmt.Sprintf("%05d", dancer.ID)
		surname, name, patronymic := parseDancerName(dancer.Title)
		dancers[i].Name = name
		dancers[i].Surname = surname
		dancers[i].Title = ""

		if dancers[i].Sex == "м" {
			dancers[i].Sex = "m"
		} else if dancers[i].Sex == "ж" {
			dancers[i].Sex = "f"
		} else {
			CheckErr(errors.New("bad sex "+dancers[i].Sex), "")
		}

		if patronymic != nil {
			dancers[i].Patronymic = dat.NullStringFrom(*patronymic)
		}
	}

	return dancers
}

func fixClubs(clubs []model.RawClub) []model.RawClub {
	maxClubId := findMaxClubId(clubs)
	clubs = append(clubs, model.RawClub{ID: 0, Name: "самост."})
	clubs = append(clubs, model.RawClub{ID: maxClubId + 1, Name: "Magnit"})
	clubs = append(clubs, model.RawClub{ID: maxClubId + 2, Name: "Intensity (г.Иваново)"})
	clubs = append(clubs, model.RawClub{ID: maxClubId + 3, Name: "Мартэ"})

	return clubs
}

func findMaxClubId(clubs []model.RawClub) int64 {
	var maxId int64 = clubs[0].ID
	for _, club := range clubs {
		if club.ID > maxId {
			maxId = club.ID
		}
	}
	return maxId
}

func fixDancerClubs(original []model.RawDancerClub, name2club map[string]int64) []model.RawDancerClub {
	dancerClubs := make([]model.RawDancerClub, 0, len(original)*2)
	for _, dc := range original {
		names := strings.Split(dc.ClubNames, ",")

		generated := generateDancerClubs(names, name2club, dc)

		dancerClubs = append(dancerClubs, generated...)
	}

	return dancerClubs
}
func generateDancerClubs(names []string, name2club map[string]int64, original model.RawDancerClub) []model.RawDancerClub {
	if len(names) == 1 {
		clubId, ok := name2club[strings.ToLower(names[0])]
		if !ok {
			log.Panic("Not found club name " + names[0])
		}
		original.ClubId = clubId
		return []model.RawDancerClub{original}
	}

	dancerClubs := make([]model.RawDancerClub, 0)
	for _, name := range names {
		club, ok := name2club[strings.ToLower(name)]
		if !ok {
			log.Panicf("Not found club name '%s', %+v", name, original)
		}

		dancerClub := model.RawDancerClub{ClubId: club, DancerID: original.DancerID, ClubNames: name}
		dancerClubs = append(dancerClubs, dancerClub)
	}

	return dancerClubs
}

func fillName2Club(name2club map[string]int64, clubs []model.RawClub) {
	for _, club := range clubs {
		name2club[strings.ToLower(club.Name)] = club.ID
	}
}

func loadFromJSON(fileName string, v interface{}) {
	data, err := ioutil.ReadFile(fileName)
	util.CheckErr(err, "Read file: "+fileName)

	json.Unmarshal(data, v)
}

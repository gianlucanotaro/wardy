package taxonomy

type Season string

const (
	SeasonSpring Season = "spring"
	SeasonSummer Season = "summer"
	SeasonAutumn Season = "autumn"
	SeasonWinter Season = "winter"
)

var Seasons = []Season{SeasonSpring, SeasonSummer, SeasonAutumn, SeasonWinter}

func (s Season) Valid() bool {
	for _, v := range Seasons {
		if s == v {
			return true
		}
	}
	return false
}

type Occasion string

const (
	OccasionWork    Occasion = "work"
	OccasionCasual  Occasion = "casual"
	OccasionFormal  Occasion = "formal"
	OccasionDate    Occasion = "date"
	OccasionSport   Occasion = "sport"
	OccasionOutdoor Occasion = "outdoor"
	OccasionHome    Occasion = "home"
)

var Occasions = []Occasion{
	OccasionWork, OccasionCasual, OccasionFormal, OccasionDate,
	OccasionSport, OccasionOutdoor, OccasionHome,
}

func (o Occasion) Valid() bool {
	for _, v := range Occasions {
		if o == v {
			return true
		}
	}
	return false
}

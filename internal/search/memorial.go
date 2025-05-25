package search

type Memorial struct {
	IsFamous               bool                    `json:"isFamous"`
	HasFlowers             bool                    `json:"hasFlowers"`
	FileName               string                  `json:"fileName,omitempty"`
	BirthCountryName       string                  `json:"birthCountryName,omitempty"`
	ManagerIsRelated       bool                    `json:"managerIsRelated"`
	BirthCirca             bool                    `json:"birthCirca"`
	PhotoContributorCounts []PhotoContributorCount `json:"photoContributorCounts,omitempty"`
	CemeteryHasPhoto       bool                    `json:"cemeteryHasPhoto"`
	BirthCityID            int                     `json:"birthCityId,omitempty"`
	DeathCountryAbbrev     string                  `json:"deathCountryAbbrev,omitempty"`
	BirthStateAbbrev       string                  `json:"birthStateAbbrev,omitempty"`
	DeathCountryID         int                     `json:"deathCountryId,omitempty"`
	HasPlot                bool                    `json:"hasPlot"`
	PersonHasPhoto         bool                    `json:"personHasPhoto"`
	IsMemorial             bool                    `json:"isMemorial"`
	NoMorePhotos           bool                    `json:"noMorePhotos"`
	BirthCountryID         int                     `json:"birthCountryId,omitempty"`
	DeathStateName         string                  `json:"deathStateName,omitempty"`
	PhotoContributors      []int                   `json:"photoContributors,omitempty"`
	PendingFamous          bool                    `json:"pendingFamous"`
	ShowSponsor            bool                    `json:"showSponsor"`
	BirthCountryAbbrev     string                  `json:"birthCountryAbbrev,omitempty"`
	NickName               string                  `json:"nickName,omitempty"`
	BirthCountyName        string                  `json:"birthCountyName,omitempty"`
	DeathStateAbbrev       string                  `json:"deathStateAbbrev,omitempty"`
	DeathCountryName       string                  `json:"deathCountryName,omitempty"`
	BirthMonth             int                     `json:"birthMonth,omitempty"`
	FirstName              string                  `json:"firstName"`
	IntermentIsFamous      bool                    `json:"intermentIsFamous"`
	DeathCityName          string                  `json:"deathCityName,omitempty"`
	DeathCountyID          int                     `json:"deathCountyId,omitempty"`
	DeathCountyName        string                  `json:"deathCountyName,omitempty"`
	DeathCirca             bool                    `json:"deathCirca"`
	NameID                 int                     `json:"nameId"`
	PersonID               int                     `json:"personId"`
	IsCenotaph             bool                    `json:"isCenotaph"`
	LastName               string                  `json:"lastName"`
	IntermentIsSponsored   bool                    `json:"intermentIsSponsored"`
	IsVeteran              bool                    `json:"isVeteran"`
	PhotoRequestNotes      bool                    `json:"photoRequestNotes"`
	DeathMonth             int                     `json:"deathMonth,omitempty"`
	PendingMerge           bool                    `json:"pendingMerge"`
	PhotoRequest           string                  `json:"photoRequest"`
	DeathCityID            int                     `json:"deathCityId,omitempty"`
	MemorialContributorID  int                     `json:"memorialContributorId"`
	DispositionLong        string                  `json:"dispositionLong,omitempty"`
	TotalImageCount        int                     `json:"totalImageCount"`
	ApprovalStatus         string                  `json:"approvalStatus"`
	MemorialID             int64                   `json:"memorialId"`
	BirthDay               int                     `json:"birthDay,omitempty"`
	IntermentHasPhoto      bool                    `json:"intermentHasPhoto"`
	FullName               string                  `json:"fullName"`
	BirthStateID           int                     `json:"birthStateId,omitempty"`
	IntermentHasFlowers    bool                    `json:"intermentHasFlowers"`
	DateModified           string                  `json:"dateModified"`
	DeathDay               int                     `json:"deathDay,omitempty"`
	DeathStateID           int                     `json:"deathStateId,omitempty"`
	BirthStateName         string                  `json:"birthStateName,omitempty"`
	IndexTimestamp         string                  `json:"indexTimestamp"`
	Disposition            string                  `json:"disposition,omitempty"`
	ApStat                 int                     `json:"apStat"`
	BirthYear              int                     `json:"birthYear,omitempty"`
	DeathYear              int                     `json:"deathYear,omitempty"`
	CreatorContributorID   int                     `json:"creatorContributorId"`
	MiddleName             string                  `json:"middleName,omitempty"`
	IntermentDateCreated   string                  `json:"IntermentDateCreated"`
	BirthCityName          string                  `json:"birthCityName,omitempty"`
	BirthCountyID          int                     `json:"birthCountyId,omitempty"`
	BirthDate              string                  `json:"birthDate"`
	DeathDate              string                  `json:"deathDate"`
	MemorialName           string                  `json:"memorialName"`
	TitleName              string                  `json:"titleName"`
	NameForURL             string                  `json:"nameForURL"`

	// Cemetery-related fields
	CemeteryStateAbbrev string `json:"cemeteryStateAbbrev,omitempty"`
	StateName           string `json:"stateName,omitempty"`
	CemeteryCountyID    int    `json:"cemeteryCountyId,omitempty"`
	CemeteryCountryName string `json:"cemeteryCountryName,omitempty"`
	CemeteryStateName   string `json:"cemeteryStateName,omitempty"`
	CountryName         string `json:"countryName,omitempty"`
	CemeteryCountryID   int    `json:"cemeteryCountryId,omitempty"`
	CityName            string `json:"cityName,omitempty"`
	CemeteryCityName    string `json:"cemeteryCityName,omitempty"`
	CemeteryStateID     int    `json:"cemeteryStateId,omitempty"`
	CountyName          string `json:"countyName,omitempty"`
	CemeteryName        string `json:"cemeteryName,omitempty"`
	CemeteryCountyName  string `json:"cemeteryCountyName,omitempty"`
	CemeteryID          int    `json:"cemeteryId,omitempty"`
	CemeteryCityID      int    `json:"cemeteryCityId,omitempty"`
	CemeteryNameForURL  string `json:"cemeteryNameForURL,omitempty"`

	// Geographic coordinates
	Longitude float64   `json:"longitude,omitempty"`
	Latitude  float64   `json:"latitude,omitempty"`
	Location  []float64 `json:"location,omitempty"`

	// Plot information
	Plot string `json:"plot,omitempty"`

	// Family relationships
	Parents  []string `json:"Parents,omitempty"`
	Spouses  []string `json:"Spouses,omitempty"`
	Children []string `json:"Children,omitempty"`
	Siblings []string `json:"Siblings,omitempty"`

	// Maiden name
	MaidenName string `json:"maidenName,omitempty"`

	// Related contributors
	RelatedContributors []RelatedContributor `json:"relatedContributors,omitempty"`

	// Limited memorial flag
	ShowLimitedMemorial bool `json:"showLimitedMemorial,omitempty"`

	// Country abbreviations
	CemeteryCountryAbbrev string `json:"cemeteryCountryAbbrev,omitempty"`
}

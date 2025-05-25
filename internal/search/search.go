package search

type SearchParams struct {
	Ajax      bool
	DeathYear int
	Page      int
	Limit     int
	Skip      int
	FName     *string
	LName     *string
}

type SearchResponse struct {
	TooMany        bool       `json:"tooMany"`
	Total          int        `json:"total"`
	Loadmore       Loadmore   `json:"loadmore"`
	Skip           int        `json:"skip"`
	Limit          int        `json:"limit"`
	Page           int        `json:"page"`
	Pages          int        `json:"pages"`
	NextURL        bool       `json:"nextURL"`
	Records        []Memorial `json:"records"`
	Collection     []Memorial `json:"collection"`
	HasQueryParams bool       `json:"hasQueryParams"`
	DeathYear      string     `json:"deathYear"`
	Location       string     `json:"location"`
	SearchURL      string     `json:"searchUrl"`
	// Honoring       Honoring   `json:"honoring"`
	QueryExecuted bool   `json:"queryExecuted"`
	ResponseCode  int    `json:"responseCode"`
	CountRecord   int    `json:"countRecord"`
	State         string `json:"state"`
	Country       string `json:"country"`
	AncestryLogin bool   `json:"ancestryLogin"`
	CurrentView   string `json:"currentView"`
}

type Loadmore struct {
	Next int `json:"next"`
	Prev int `json:"prev"`
}

type PhotoContributorCount struct {
	Count              int  `json:"count"`
	PhotoContributorID int  `json:"photoContributorId"`
	IsSponsor          bool `json:"isSponsor"`
}

type RelatedContributor struct {
	ContributorID int    `json:"contributorId"`
	IsPublic      bool   `json:"isPublic"`
	Relationship  string `json:"relationship"`
}

type Honoring struct {
	DateModified *string `json:"dateModified"`
	IntermentID  int     `json:"intermentId"`
	FirstName    string  `json:"firstName"`
	LastName     string  `json:"lastName"`
	PhotoFile    string  `json:"photoFile"`
	DeathYear    int     `json:"deathYear"`
	BirthYear    int     `json:"birthYear"`
	BirthDate    string  `json:"birthDate"`
	DeathDate    string  `json:"deathDate"`
	MemorialName string  `json:"memorialName"`
	TitleName    string  `json:"titleName"`
	NameForURL   string  `json:"nameForURL"`
}

func (m *Memorial) IsAnimalPet() bool {
	return m.Disposition == "Animal/Pet" || m.DispositionLong == "Animal/Pet"
}

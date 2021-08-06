package util

type JobProfile struct {
	CompanyUrl        string
	Source            string
	PostDate          string
	CompanyStaff      string
	EnglishName       string
	ChineseName       string
	CompanyShort      string
	LegalEntity       string
	FoundingDay       string
	RegisteredCapital string
	ContactPerson     string
	ContactPosition   string
	JobTitle          string
	MinSalary         string
	MaxSalary         string
	SalaryLimit       string
	HireNumber        string
	Experience        string
	Education         string
	Realm             string
	Location          string
	OperatingItems    string
}

var JobsChan = make(chan JobProfile, 10)

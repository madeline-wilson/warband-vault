package buildinfo

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
	Channel   = "development"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	Channel   string `json:"channel"`
}

func Current() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		Channel:   Channel,
	}
}

func String() string {
	return fmt.Sprintf("Warband Vault %s commit=%s built=%s channel=%s", Version, Commit, BuildDate, Channel)
}

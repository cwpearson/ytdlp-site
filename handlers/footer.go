package handlers

import "ytdlp-site/config"

type Footer struct {
	BuildDate    string
	BuildId      string
	BuildIdShort string
}

func MakeFooter() Footer {
	return Footer{
		BuildDate:    config.GetBuildDate(),
		BuildId:      config.GetGitSHA(),
		BuildIdShort: config.GetGitSHA()[0:7],
	}
}

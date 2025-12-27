package ui

import (
	"time"

	"git.punjwani.pm/Mattia/DepthTUI/internal/api"
	"git.punjwani.pm/Mattia/DepthTUI/internal/player"
	tea "github.com/charmbracelet/bubbletea"
)

func searchCmd(query string, mode int) tea.Cmd {
	return func() tea.Msg {

		switch mode {
		case filterSongs:
			songs, err := api.SubsonicSearchSong(query, 0)
			if err != nil {
				return errMsg{err}
			}
			return songsResultMsg{songs}

		case filterAlbums:
			// Ensure api.SubsonicSearchAlbum exists in your api package!
			albums, err := api.SubsonicSearchAlbum(query, 0)
			if err != nil {
				return errMsg{err}
			}
			return albumsResultMsg{albums}

		case filterArtist:
			// Ensure api.SubsonicSearchArtist exists in your api package!
			artists, err := api.SubsonicSearchArtist(query, 0)
			if err != nil {
				return errMsg{err}
			}
			return artistsResultMsg{artists}
		}

		return nil
	}
}
func getPlaylists() tea.Cmd {
	return func() tea.Msg {
		playlists, err := api.SubsonicGetPlaylists()
		if err != nil {
			return errMsg{err}
		}
		return playlistResultMsg{playlists}
	}
}

func getPlaylistSongs(id string) tea.Cmd {
	return func() tea.Msg {
		songs, err := api.SubsonicGetPlaylistSongs(id)
		if err != nil {
			return errMsg{err}
		}
		return songsResultMsg{songs}
	}
}

func syncPlayerCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return statusMsg(player.GetPlayerStatus())
	})
}

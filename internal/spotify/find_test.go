package spotify

import "testing"

func TestFindPlaylists(t *testing.T) {
	lib := []Playlist{
		{ID: "a", Name: "Tristes 😢😓"},
		{ID: "b", Name: "Covers"},
		{ID: "c", Name: "80s cover"},
		{ID: "d", Name: "Banda 🤠"},
		{ID: "e", Name: "LadoB🤠"},
	}
	cases := []struct {
		query string
		want  []string
	}{
		{"Tristes", []string{"a"}},     // partial match ignores the emoji suffix
		{"  tristes  ", []string{"a"}}, // case and spaces ignored
		{"covers", []string{"b"}},      // exact match wins over partial ones
		{"cover", []string{"b", "c"}},  // substring of both: ambiguous
		{"🤠", []string{"d", "e"}},      // ambiguous
		{"reggaeton", nil},
		{"", nil},
	}
	for _, c := range cases {
		got := FindPlaylists(lib, c.query)
		if len(got) != len(c.want) {
			t.Errorf("FindPlaylists(%q) = %v, want ids %v", c.query, got, c.want)
			continue
		}
		for i := range got {
			if got[i].ID != c.want[i] {
				t.Errorf("FindPlaylists(%q)[%d] = %s, want %s", c.query, i, got[i].ID, c.want[i])
			}
		}
	}
}

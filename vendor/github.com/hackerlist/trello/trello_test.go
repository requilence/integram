package trello

import (
	"encoding/json"
	"io/ioutil"
	"sync"
	"testing"
)

type Creds struct {
	Key, Secret, Token, Member, Organization string
}

var (
	creds Creds
	load  sync.Once
)

func loadCreds() {
	b, err := ioutil.ReadFile("trello.json")

	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(b, &creds)

	if err != nil {
		panic(err)
	}
}

func setupTest() {
	load.Do(loadCreds)

	if creds.Key == "" {
		panic("no key")
	}
}

func TestMemberUsername(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	m, err := c.Member(creds.Member)

	if err != nil {
		t.Errorf("member request: %s", err)
	} else {
		t.Logf("%s", m.Username)
	}
}

func TestMemberFullName(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	m, err := c.Member(creds.Member)

	if err != nil {
		t.Errorf("member request: %s", err)
	} else {
		t.Logf("%s", m.FullName)
	}
}

func TestMemberBio(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	m, err := c.Member(creds.Member)

	if err != nil {
		t.Errorf("member request: %s", err)
	} else {
		t.Logf("%s", m.Bio)
	}
}

func TestMemberListCards(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	m, err := c.Member(creds.Member)
	if err != nil {
		t.Errorf("member request: %s", err)
	} else {
		if boards, err := m.Boards(); err != nil {
			t.Errorf("board request: %s", err)
		} else {
			if len(boards) > 0 {
				if lists, err := boards[0].Lists(); err != nil {
					t.Errorf("list request: %s", err)
				} else {
					if len(lists) > 0 {
						if cards, err := lists[0].Cards(); err != nil {
							t.Errorf("card request: %s", err)
						} else {
							if len(cards) > 0 {
								t.Logf("card %+v", cards[0])
							} else {
								t.Errorf("no cards")
							}
						}
					} else {
						t.Errorf("no lists")
					}
				}
			} else {
				t.Errorf("no boards")
			}
		}
	}
}

func TestMemberListCardsChecklists(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	// NotImplemented
	m, err := c.Member(creds.Member)
	if err != nil {
		t.Errorf("member request: %s", err)
	} else {
		if boards, err := m.Boards(); err != nil {
			t.Errorf("board request: %s", err)
		} else {
			for _, b := range boards {
				if b.Name == "test" {
					if lists, err := b.Lists(); err != nil {
						t.Errorf("no lists: %s", err)
					} else {
						for _, list := range lists {
							if cards, err := list.Cards(); err != nil {
								t.Errorf("card request: %s", err)
							} else {
								for _, card := range cards {
									if checklists, err := card.Checklists(); err != nil {
										t.Errorf("checklists request: %s", err)
									} else {
										for _, c := range checklists {
											t.Logf("checklist %+v", c)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestOrganizationMembers(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	o, err := c.Organization(creds.Organization)
	if err != nil {
		t.Errorf("organization request: %s", err)
	} else {
		if members, err := o.Members(); err != nil {
			t.Errorf("members request: %s", err)
		} else {
			for _, m := range members {
				t.Logf("%+v", m)
			}
		}
	}
}

func TestOrganizationBoardListsCards(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	o, err := c.Organization(creds.Organization)
	if err != nil {
		t.Errorf("organization request: %s", err)
	} else {
		if boards, err := o.Boards(); err != nil {
			t.Errorf("board request: %s", err)
		} else {
			for _, b := range boards {
				if lists, err := b.Lists(); err != nil {
					t.Errorf("list request: %s", err)
				} else {
					for _, l := range lists {
						if cards, err := l.Cards(); err != nil {
							t.Errorf("card request: %s", err)
						} else {
							for _, c := range cards {
								t.Logf("card %+v", c)
							}
						}
					}
				}
			}
		}
	}
}

func TestOrganizationCardAddComment(t *testing.T) {
	setupTest()

	c := New(creds.Key, creds.Secret, creds.Token)
	o, err := c.Organization(creds.Organization)
	if err != nil {
		t.Errorf("organization request: %s", err)
	} else {
		if boards, err := o.Boards(); err != nil {
			t.Errorf("board request: %s", err)
		} else {
			for _, b := range boards {
				if cards, err := b.Cards(); err != nil {
					t.Errorf("card request: %s", err)
				} else {
					for _, c := range cards {
						if c.Name == "test" {
							if err := c.AddComment("test"); err != nil {
								t.Errorf("addcomment error: %s", err)
							}
						}
					}
				}
			}
		}
	}
}

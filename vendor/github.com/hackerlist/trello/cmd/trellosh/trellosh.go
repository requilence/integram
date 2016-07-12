package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/hackerlist/trello"
)

type Creds struct {
	Key, Secret, Token, Member, Organization string
}

var (
	creds Creds
	c     *trello.Client
	load  sync.Once
)

func loadCreds() error {
	b, err := ioutil.ReadFile(os.Getenv("HOME") + "/.trelloshrc")

	if err != nil {
		fmt.Printf("No credentials found.\n")
		return err
	}

	err = json.Unmarshal(b, &creds)

	if err != nil {
		fmt.Printf("Invalid credentials.\n")
		return err
	}

	//creds.Key = "09f16319e72a2488397b119be7560215"
	return nil
}

func main() {
	flag.Parse()
	loadCreds()

	c = trello.New(creds.Key, creds.Secret, creds.Token)

	sc := bufio.NewScanner(os.Stdin)

	me, err := c.Member(creds.Member)

	if err != nil {
		fmt.Printf("error loading me: %s", err)
		return
	}

	boardsf := func() {
		if boards, err := me.Boards(); err != nil {
			fmt.Printf("error: %s\n", err)
		} else {
			boardsprint(boards)
		}
	}

	fmt.Printf("trellosh> ")
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 1 {
			boardsf()
		} else {
			switch f[0] {
			case "boards":
				boardsf()
			case "board":
				if len(f) > 1 {
					boardrepl(f[1], sc)
				} else {
					fmt.Println("missing board argument\n")
				}
			case "createboard":
				if len(f) > 1 {
					if b, err := c.CreateBoard(f[1], nil); err != nil {
						fmt.Printf("createboard error: %s\n", err)
					} else {
						boardsprint([]*trello.Board{b})
					}
				}
			default:
				fallthrough
			case "help":
				fmt.Printf("commands:\n")
				for _, cmd := range []string{"boards", "board id", "createboard name"} {
					fmt.Printf("  %s\n", cmd)
				}
			case "exit":
				return
			}
		}
		fmt.Printf("trellosh> ")
	}
}

func boardsprint(boards []*trello.Board) {
	fmt.Printf("%-24.24s %-20.20s %-24.24s\n", "id", "name", "shorturl")
	for _, b := range boards {
		fmt.Printf("%24.24s %-20.20s %-24.24s\n", b.Id, b.Name, b.ShortUrl)
	}
}

func boardrepl(id string, sc *bufio.Scanner) error {
	board, err := c.Board(id)
	if err != nil {
		fmt.Printf("load board error: %s\n", err)
		return err
	}

	var last func()

	lists := func() {
		if lists, err := board.Lists(); err != nil {
			fmt.Printf("lists error: %s\n", err)
		} else {
			listsprint(lists)
		}
	}

	last = lists

	last()

	fmt.Printf("board %s> ", board.Name)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 1 {
			if last != nil {
				last()
			}
		} else {
			switch f[0] {
			case "lists":
				last = lists
				last()
			case "list":
				if len(f) > 1 {
					listrepl(f[1], sc)
				} else {
					fmt.Println("usage: list id\n")
				}
			case "addlist":
				if len(f) > 1 {
					if list, err := board.AddList(f[1]); err != nil {
						fmt.Printf("addlist error: %s\n", err)
					} else {
						listsprint([]*trello.List{list})
					}
				} else {
					fmt.Println("usage: newlist name\n")
				}
			case "members":
				last = func() {
					if members, err := board.Members(); err != nil {
						fmt.Printf("members error: %s\n", err)
					} else {
						membersprint(members)
					}
				}
				last()
			case "invite":
				if len(f) > 3 {
					if err := board.Invite(f[1], strings.Join(f[3:], " "), f[2]); err != nil {
						fmt.Printf("invite error: %s\n", err)
					}
				} else {
					fmt.Println("usage: invite email type fullname...\n")
					fmt.Println("type may be one of: normal observer admin\n")
				}
			case "addmember":
				if len(f) > 2 {
					if err := board.AddMember(f[1], f[2]); err != nil {
						fmt.Printf("addmember error: %s\n", err)
					}
				} else {
					fmt.Println("usage: addmember id type...\n")
					fmt.Println("type may be one of: normal observer admin\n")
				}
			default:
				fallthrough
			case "help":
				fmt.Printf("commands:\n")
				for _, cmd := range []string{"lists", "list id", "addlist name", "invite email type fullname...", "addmember id type"} {
					fmt.Printf("  %s\n", cmd)
				}
			case "exit":
				return nil
			}
		}
		fmt.Printf("board %s> ", board.Name)
	}
	return sc.Err()
}

func listsprint(lists []*trello.List) {
	fmt.Printf("%-24.24s %-20.20s\n", "id", "name")
	for _, l := range lists {
		fmt.Printf("%-24.24s %-20.20s\n", l.Id, l.Name)
	}
}

func listrepl(id string, sc *bufio.Scanner) error {
	list, err := c.List(id)
	if err != nil {
		fmt.Printf("load list error: %s\n", err)
		return err
	}

	var last func()

	cards := func() {
		if cards, err := list.Cards(); err != nil {
			fmt.Printf("cards error: %s\n", err)
		} else {
			cardsprint(cards)
		}
	}

	last = cards

	last()

	fmt.Printf("list %s> ", list.Name)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 1 {
			if last != nil {
				last()
			}
		} else {
			switch f[0] {
			case "cards":
				last = cards
				last()
			case "card":
				if len(f) > 1 {
					cardrepl(f[1], sc)
				} else {
					fmt.Println("missing card argument\n")
				}
			case "addcard":
				if len(f) > 1 {
					if ca, err := list.AddCard(f[1], nil); err != nil {
						fmt.Printf("addcard error: %s\n", err)
					} else {
						cardsprint([]*trello.Card{ca})
					}
				}
			default:
				fallthrough
			case "help":
				fmt.Printf("commands:\n")
				for _, cmd := range []string{"cards", "card id", "addcard name"} {
					fmt.Printf("  %s\n", cmd)
				}
			case "exit":
				return nil
			}
		}
		fmt.Printf("list %s> ", list.Name)
	}
	return sc.Err()
}

func cardsprint(cards []*trello.Card) {
	fmt.Printf("%-24.24s %-20.20s\n", "id", "name")
	for _, c := range cards {
		fmt.Printf("%-24.24s %-20.20s\n", c.Id, c.Name)
	}
}

func cardrepl(id string, sc *bufio.Scanner) error {
	card, err := c.Card(id)
	if err != nil {
		fmt.Printf("load card error: %s\n", err)
		return err
	}

	var last func()

	actions := func() {
		if actions, err := card.Actions(); err != nil {
			fmt.Printf("cards error: %s\n", err)
		} else {
			actionsprint(actions)
		}
	}

	checklist := func() {
		if checklists, err := card.Checklists(); err != nil {
			fmt.Printf("checklist error: %s\n", err)
		} else {
			checklistsprint(checklists)
		}
	}

	last = actions

	last()

	fmt.Printf("card %s> ", card.Name)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 1 {
			if last != nil {
				last()
			}
		} else {
			switch f[0] {
			case "comment":
				if len(f) > 1 {
					card.AddComment(strings.Join(f[1:], " "))
				} else {
					fmt.Printf("usage: comment text...\n")
				}
			case "actions":
				last = actions
				last()
			case "checklists":
				last = checklist
				last()
			case "checklist":
				if len(f) > 1 {
					checklistrepl(f[1], sc)
				} else {
					fmt.Printf("usage: checklist id\n")
				}
			case "addchecklist":
				if len(f) > 1 {
					if cl, err := card.AddChecklist(f[1]); err != nil {
						fmt.Printf("addchecklist error: %s\n", err)
					} else {
						checklistsprint([]*trello.Checklist{cl})
					}
				} else {
					fmt.Printf("usage: addchecklist name\n")
				}
			default:
				fallthrough
			case "help":
				fmt.Printf("commands:\n")
				for _, cmd := range []string{"actions", "comment text", "checklists", "checklist id", "addchecklist name"} {
					fmt.Printf("  %s\n", cmd)
				}
			case "exit":
				return nil
			}
		}
		fmt.Printf("card %s> ", card.Name)
	}
	return sc.Err()
}

func actionsprint(actions []*trello.Action) {
	fmt.Printf("%-24.24s %-20.20s %-20.20s\n", "id", "type", "text")
	for _, a := range actions {
		fmt.Printf("%-24.24s %-20.20s %-20.20s\n", a.Id, a.Type, a.Data.Text)
	}
}

func checklistsprint(checklists []*trello.Checklist) {
	fmt.Printf("%-24.24s %-20.20s\n", "id", "name")
	for _, cl := range checklists {
		fmt.Printf("%-24.24s %-20.20s\n", cl.Id, cl.Name)
	}
}

func checkitemsprint(checkitems []*trello.CheckItem) {
	fmt.Printf("%-24.24s %-20.20s %-10.10s\n", "id", "name", "state")
	for _, cl := range checkitems {
		fmt.Printf("%-24.24s %-20.20s %-10.10s\n", cl.Id, cl.Name, cl.State)
	}

}

func checklistrepl(id string, sc *bufio.Scanner) error {
	checklist, err := c.Checklist(id)
	if err != nil {
		fmt.Printf("load checlist error: %s\n", err)
		return err
	}

	var last func()

	checklistf := func() {
		if checklist, err = c.Checklist(id); err != nil {
			fmt.Printf("checklist error: %s\n", err)
		} else {
			checkitemsprint(checklist.CheckItems)
		}
	}

	last = checklistf

	last()

	fmt.Printf("checklist %s> ", checklist.Name)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 1 {
			if last != nil {
				last()
			}
		} else {
			switch f[0] {
			case "additem":
				if len(f) > 1 {
					checklist.AddItem(strings.Join(f[1:], " "))
				} else {
					fmt.Printf("usage: additem name...\n")
				}
				checklistf()
			case "checkitems":
				last = checklistf
				last()
			case "uncheck":
				fallthrough
			case "check":
				if len(f) > 1 {
					for _, ci := range checklist.CheckItems {
						if ci.Id == f[1] {
							switch f[0] {
							case "check":
								err = checklist.CheckItem(ci.Id, true)
							case "uncheck":
								err = checklist.CheckItem(ci.Id, false)
							}
							if err != nil {
								fmt.Printf("%s error: %s\n", f[0], err)
							}
						}
					}
				} else {
					fmt.Printf("usage: %s id\n", f[0])
				}

			default:
				fallthrough
			case "help":
				fmt.Printf("commands:\n")
				for _, cmd := range []string{"additem name...", "checkitems", "check id", "uncheck id"} {
					fmt.Printf("  %s\n", cmd)
				}
			case "exit":
				return nil
			}
		}
		if checklist != nil {
			fmt.Printf("checklist %s> ", checklist.Name)
		} else {
			return fmt.Errorf("checklist gone")
		}
	}
	return sc.Err()
}

func membersprint(members []*trello.Member) {
	fmt.Printf("%-24.24s %-20.20s %-20.20s\n", "id", "username", "fullname")
	for _, u := range members {
		fmt.Printf("%-24.24s %-20.20s %-20.20s\n", u.Id, u.Username, u.FullName)
	}
}

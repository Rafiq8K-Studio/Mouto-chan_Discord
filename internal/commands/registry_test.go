package commands

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

var called = ""

func testModule(prefix string) Module {
	return Module{
		Command:         &discordgo.ApplicationCommand{Name: "t" + prefix},
		Slash:           func(*discordgo.Session, *discordgo.InteractionCreate) {},
		ModalPrefix:     prefix + ":modal:",
		Modal:           func(*discordgo.Session, *discordgo.InteractionCreate) { called = prefix + "-modal" },
		ComponentPrefix: prefix + ":comp:",
		Component:       func(*discordgo.Session, *discordgo.InteractionCreate) { called = prefix + "-comp" },
	}
}

func TestFindModal(t *testing.T) {
	mods := []Module{testModule("a"), testModule("b")}
	h := findModal(mods, "a:modal:123")
	if h == nil {
		t.Fatal("handler modal 'a' tidak ketemu")
	}
	h(nil, nil)
	if called != "a-modal" {
		t.Fatalf("called=%q", called)
	}
	if findModal(mods, "tak-ada:modal:1") != nil {
		t.Fatal("harus nil untuk prefix tak dikenal")
	}
}

func TestFindComponent(t *testing.T) {
	mods := []Module{testModule("a"), testModule("b")}
	h := findComponent(mods, "b:comp:u1:choice")
	if h == nil {
		t.Fatal("handler component 'b' tidak ketemu")
	}
	h(nil, nil)
	if called != "b-comp" {
		t.Fatalf("called=%q", called)
	}
}

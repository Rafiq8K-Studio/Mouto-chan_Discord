package ping

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

// Command mendefinisikan slash command /ping.
func Command() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Cek latency bot",
	}
}

// Handler membalas interaction dengan Pong + latency websocket.
func Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong! 🏓 (ws latency: " + s.HeartbeatLatency().Round(time.Millisecond).String() + ")",
		},
	})
}

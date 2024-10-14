package skippy

import "fmt"

func joinVoice(dg DiscordSession, guildID string, userID string, state *State) error {

	guild, err := dg.GetState().Guild(guildID)
	if err != nil {
		return fmt.Errorf("error could not join voice %w", err)
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			// join voice
			vc, err := dg.ChannelVoiceJoin(guildID, vs.ChannelID, false, false)
			if err != nil {
				return fmt.Errorf("error could not join voice channel %w", err)
			}
			state.AddVoiceConnection(vs.ChannelID, vc)
		}
	}

	return nil
}

func leaveVoice(dg DiscordSession, guildID string, userID string, state *State) error {

	guild, err := dg.GetState().Guild(guildID)
	if err != nil {
		return fmt.Errorf("error could not join voice %w", err)
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			vc := state.GetVoiceConnection(vs.ChannelID)
			if vc == nil {
				return fmt.Errorf("error could not leave voice channel as it does not exist in state")
			}
			err := vc.Disconnect()
			if err != nil {
				return fmt.Errorf("error could not leave voice %w", err)
			}

			if vc.OpusRecv != nil {
				close(vc.OpusRecv)
			}

			vc.Close()
			state.RemoveVoiceConnection(vs.ChannelID)
		}
	}

	return nil
}

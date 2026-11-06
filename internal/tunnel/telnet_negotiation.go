package tunnel

import "fmt"

const (
	telnetIAC  byte = 255
	telnetDONT byte = 254
	telnetDO   byte = 253
	telnetWONT byte = 252
	telnetWILL byte = 251
	telnetSB   byte = 250

	telnetGA  byte = 249
	telnetNOP byte = 241
	telnetSE  byte = 240
)

const (
	telnetOptBinary       byte = 0
	telnetOptEcho         byte = 1
	telnetOptSuppressGA   byte = 3
	telnetOptTerminalType byte = 24
	telnetOptNAWS         byte = 31
)

const (
	telnetTerminalTypeIs   byte = 0
	telnetTerminalTypeSend byte = 1
)

type telnetCommand struct {
	verb   byte
	option byte
}

type telnetSubnegotiation struct {
	option  byte
	payload []byte
}

type telnetParserState uint8

const (
	telnetParserData telnetParserState = iota
	telnetParserIAC
	telnetParserCommand
	telnetParserSubnegotiation
	telnetParserSubnegotiationIAC
)

type telnetParser struct {
	state telnetParserState

	command byte

	subOption    byte
	subHasOption bool
	subData      []byte
}

func (p *telnetParser) feed(chunk []byte, data []byte) ([]byte, []telnetCommand, []telnetSubnegotiation) {
	var commands []telnetCommand
	var subs []telnetSubnegotiation
	for _, b := range chunk {
		switch p.state {
		case telnetParserData:
			if b == telnetIAC {
				p.state = telnetParserIAC
				continue
			}
			data = append(data, b)
		case telnetParserIAC:
			switch b {
			case telnetIAC:

				data = append(data, telnetIAC)
				p.state = telnetParserData
			case telnetWILL, telnetWONT, telnetDO, telnetDONT:
				p.command = b
				p.state = telnetParserCommand
			case telnetSB:
				p.subOption = 0
				p.subHasOption = false
				p.subData = p.subData[:0]
				p.state = telnetParserSubnegotiation
			default:

				p.state = telnetParserData
			}
		case telnetParserCommand:
			commands = append(commands, telnetCommand{verb: p.command, option: b})
			p.state = telnetParserData
		case telnetParserSubnegotiation:
			if b == telnetIAC {
				p.state = telnetParserSubnegotiationIAC
				continue
			}
			if !p.subHasOption {
				p.subOption = b
				p.subHasOption = true
				continue
			}
			p.subData = append(p.subData, b)
		case telnetParserSubnegotiationIAC:
			switch b {
			case telnetSE:
				subs = append(subs, telnetSubnegotiation{
					option:  p.subOption,
					payload: append([]byte(nil), p.subData...),
				})
				p.state = telnetParserData
			case telnetIAC:

				p.subData = append(p.subData, telnetIAC)
				p.state = telnetParserSubnegotiation
			default:

				p.subData = append(p.subData, b)
				p.state = telnetParserSubnegotiation
			}
		}
	}
	return data, commands, subs
}

type telnetNegotiationState uint8

const (
	telnetStateNo telnetNegotiationState = iota
	telnetStateWantNo
	telnetStateWantYes
	telnetStateYes
)

type telnetOptionSide struct {
	state telnetNegotiationState

	queued bool
}

type telnetNegotiator struct {
	us  [256]telnetOptionSide
	him [256]telnetOptionSide

	acceptLocal  [256]bool
	acceptRemote [256]bool

	anomalies int
}

func newTelnetNegotiator() *telnetNegotiator {
	n := &telnetNegotiator{}
	for _, option := range []byte{
		telnetOptNAWS,
		telnetOptTerminalType,
		telnetOptSuppressGA,
		telnetOptBinary,
	} {
		n.acceptLocal[option] = true
	}
	for _, option := range []byte{
		telnetOptEcho,
		telnetOptSuppressGA,
		telnetOptBinary,
	} {
		n.acceptRemote[option] = true
	}
	return n
}

func (n *telnetNegotiator) receive(verb byte, option byte) []telnetCommand {
	switch verb {
	case telnetWILL:
		return n.receiveRemoteEnable(option)
	case telnetWONT:
		return n.receiveRemoteDisable(option)
	case telnetDO:
		return n.receiveLocalEnable(option)
	case telnetDONT:
		return n.receiveLocalDisable(option)
	default:
		return nil
	}
}

func (n *telnetNegotiator) receiveRemoteEnable(option byte) []telnetCommand {
	side := &n.him[option]
	switch side.state {
	case telnetStateNo:
		if n.acceptRemote[option] {
			side.state = telnetStateYes
			return []telnetCommand{{verb: telnetDO, option: option}}
		}
		return []telnetCommand{{verb: telnetDONT, option: option}}
	case telnetStateYes:

		return nil
	case telnetStateWantNo:
		n.anomalies++
		if !side.queued {

			side.state = telnetStateNo
			return nil
		}
		side.state = telnetStateYes
		side.queued = false
		return nil
	default:
		if !side.queued {
			side.state = telnetStateYes
			return nil
		}
		side.state = telnetStateWantNo
		side.queued = false
		return []telnetCommand{{verb: telnetDONT, option: option}}
	}
}

func (n *telnetNegotiator) receiveRemoteDisable(option byte) []telnetCommand {
	side := &n.him[option]
	switch side.state {
	case telnetStateNo:

		return nil
	case telnetStateYes:
		side.state = telnetStateNo
		return []telnetCommand{{verb: telnetDONT, option: option}}
	case telnetStateWantNo:
		if !side.queued {
			side.state = telnetStateNo
			return nil
		}
		side.state = telnetStateWantYes
		side.queued = false
		return []telnetCommand{{verb: telnetDO, option: option}}
	default:

		side.state = telnetStateNo
		side.queued = false
		return nil
	}
}

func (n *telnetNegotiator) receiveLocalEnable(option byte) []telnetCommand {
	side := &n.us[option]
	switch side.state {
	case telnetStateNo:
		if n.acceptLocal[option] {
			side.state = telnetStateYes
			return []telnetCommand{{verb: telnetWILL, option: option}}
		}
		return []telnetCommand{{verb: telnetWONT, option: option}}
	case telnetStateYes:
		return nil
	case telnetStateWantNo:
		n.anomalies++
		if !side.queued {
			side.state = telnetStateNo
			return nil
		}
		side.state = telnetStateYes
		side.queued = false
		return nil
	default:
		if !side.queued {
			side.state = telnetStateYes
			return nil
		}
		side.state = telnetStateWantNo
		side.queued = false
		return []telnetCommand{{verb: telnetWONT, option: option}}
	}
}

func (n *telnetNegotiator) receiveLocalDisable(option byte) []telnetCommand {
	side := &n.us[option]
	switch side.state {
	case telnetStateNo:
		return nil
	case telnetStateYes:
		side.state = telnetStateNo
		return []telnetCommand{{verb: telnetWONT, option: option}}
	case telnetStateWantNo:
		if !side.queued {
			side.state = telnetStateNo
			return nil
		}
		side.state = telnetStateWantYes
		side.queued = false
		return []telnetCommand{{verb: telnetWILL, option: option}}
	default:
		side.state = telnetStateNo
		side.queued = false
		return nil
	}
}

func (n *telnetNegotiator) requestLocalEnable(option byte) []telnetCommand {
	return n.requestEnable(&n.us[option], telnetWILL, option)
}

func (n *telnetNegotiator) requestLocalDisable(option byte) []telnetCommand {
	return n.requestDisable(&n.us[option], telnetWONT, option)
}

func (n *telnetNegotiator) requestRemoteEnable(option byte) []telnetCommand {
	return n.requestEnable(&n.him[option], telnetDO, option)
}

func (n *telnetNegotiator) requestRemoteDisable(option byte) []telnetCommand {
	return n.requestDisable(&n.him[option], telnetDONT, option)
}

func (n *telnetNegotiator) requestEnable(side *telnetOptionSide, verb byte, option byte) []telnetCommand {
	switch side.state {
	case telnetStateNo:
		side.state = telnetStateWantYes
		return []telnetCommand{{verb: verb, option: option}}
	case telnetStateYes:
		n.anomalies++
		return nil
	case telnetStateWantNo:
		if !side.queued {

			side.queued = true
			return nil
		}
		n.anomalies++
		return nil
	default:
		if !side.queued {
			n.anomalies++
			return nil
		}

		side.queued = false
		return nil
	}
}

func (n *telnetNegotiator) requestDisable(side *telnetOptionSide, verb byte, option byte) []telnetCommand {
	switch side.state {
	case telnetStateNo:
		n.anomalies++
		return nil
	case telnetStateYes:
		side.state = telnetStateWantNo
		return []telnetCommand{{verb: verb, option: option}}
	case telnetStateWantNo:
		if !side.queued {
			n.anomalies++
			return nil
		}
		side.queued = false
		return nil
	default:
		if !side.queued {
			side.queued = true
			return nil
		}
		n.anomalies++
		return nil
	}
}

func (n *telnetNegotiator) localEnabled(option byte) bool {
	return n.us[option].state == telnetStateYes
}

func (n *telnetNegotiator) remoteEnabled(option byte) bool {
	return n.him[option].state == telnetStateYes
}

func appendTelnetEscaped(dst []byte, payload []byte) []byte {
	for _, b := range payload {
		dst = append(dst, b)
		if b == telnetIAC {
			dst = append(dst, telnetIAC)
		}
	}
	return dst
}

func telnetCommandFrame(verb byte, option byte) []byte {
	return []byte{telnetIAC, verb, option}
}

func telnetSubnegotiationFrame(option byte, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+5)
	frame = append(frame, telnetIAC, telnetSB, option)
	frame = appendTelnetEscaped(frame, payload)
	return append(frame, telnetIAC, telnetSE)
}

func telnetNAWSPayload(columns int, rows int) []byte {
	width := uint16(columns)
	height := uint16(rows)
	return []byte{byte(width >> 8), byte(width), byte(height >> 8), byte(height)}
}

func telnetTerminalTypePayload(name string) []byte {
	payload := make([]byte, 0, len(name)+1)
	payload = append(payload, telnetTerminalTypeIs)
	return append(payload, name...)
}

func telnetYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func telnetNegotiationSummary(negotiator *telnetNegotiator) string {
	if negotiator == nil {
		return ""
	}
	echo := "off"
	switch {
	case negotiator.remoteEnabled(telnetOptEcho):
		echo = "remote"
	case negotiator.localEnabled(telnetOptEcho):
		echo = "local"
	}
	return fmt.Sprintf("echo=%s naws=%s terminal-type=%s anomalies=%d",
		echo,
		telnetYesNo(negotiator.localEnabled(telnetOptNAWS)),
		telnetYesNo(negotiator.localEnabled(telnetOptTerminalType)),
		negotiator.anomalies)
}

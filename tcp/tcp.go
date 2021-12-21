package tcp

// IsRequest if is req of tcp packet
func IsRequest(pckt *Packet, address string) bool {
	return pckt.Dst() == address
}

// IsResponse is resp of tcp packet
func IsResponse(pckt *Packet, address string) bool {
	return pckt.Src() == address
}

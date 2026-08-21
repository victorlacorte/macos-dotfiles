package mdview

// posixCksum implements the POSIX cksum CRC-32 variant. POSIX appends the
// input length, least-significant byte first, to the byte stream before the
// final complement. This is intentionally not hash/crc32.ChecksumIEEE: the
// cache names are part of md-view's compatibility contract.
func posixCksum(data []byte) uint32 {
	var checksum uint32
	for _, value := range data {
		checksum = cksumByte(checksum, value)
	}

	length := uint64(len(data))
	for length != 0 {
		checksum = cksumByte(checksum, byte(length))
		length >>= 8
	}
	return ^checksum
}

func cksumByte(checksum uint32, value byte) uint32 {
	checksum ^= uint32(value) << 24
	for bit := 0; bit < 8; bit++ {
		if checksum&0x80000000 != 0 {
			checksum = checksum<<1 ^ 0x04c11db7
		} else {
			checksum <<= 1
		}
	}
	return checksum
}

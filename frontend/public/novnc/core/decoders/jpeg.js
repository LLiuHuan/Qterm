export default class JPEGDecoder {
  constructor() {
    this._cachedQuantTables = []
    this._cachedHuffmanTables = []

    this._segments = []
  }

  decodeRect(x, y, width, height, sock, display, depth) {
    while (true) {
      let segment = this._readSegment(sock)
      if (segment === null) {
        return false
      }
      this._segments.push(segment)

      if (segment[1] === 0xd9) {
        break
      }
    }

    let huffmanTables = []
    let quantTables = []
    for (let segment of this._segments) {
      let type = segment[1]
      if (type === 0xc4) {
        huffmanTables.push(segment)
      } else if (type === 0xdb) {
        quantTables.push(segment)
      }
    }

    const sofIndex = this._segments.findIndex((x) => x[1] == 0xc0 || x[1] == 0xc2)
    if (sofIndex == -1) {
      throw new Error('Illegal JPEG image without SOF')
    }

    if (quantTables.length === 0) {
      this._segments.splice(sofIndex + 1, 0, ...this._cachedQuantTables)
    }
    if (huffmanTables.length === 0) {
      this._segments.splice(sofIndex + 1, 0, ...this._cachedHuffmanTables)
    }

    let length = 0
    for (let segment of this._segments) {
      length += segment.length
    }

    let data = new Uint8Array(length)
    length = 0
    for (let segment of this._segments) {
      data.set(segment, length)
      length += segment.length
    }

    display.imageRect(x, y, width, height, 'image/jpeg', data)

    if (huffmanTables.length !== 0) {
      this._cachedHuffmanTables = huffmanTables
    }
    if (quantTables.length !== 0) {
      this._cachedQuantTables = quantTables
    }

    this._segments = []

    return true
  }

  _readSegment(sock) {
    if (sock.rQwait('JPEG', 2)) {
      return null
    }

    let marker = sock.rQshift8()
    if (marker != 0xff) {
      throw new Error('Illegal JPEG marker received (byte: ' + marker + ')')
    }
    let type = sock.rQshift8()
    if ((type >= 0xd0 && type <= 0xd9) || type == 0x01) {
      return new Uint8Array([marker, type])
    }

    if (sock.rQwait('JPEG', 2, 2)) {
      return null
    }

    let length = sock.rQshift16()
    if (length < 2) {
      throw new Error('Illegal JPEG length received (length: ' + length + ')')
    }

    if (sock.rQwait('JPEG', length - 2, 4)) {
      return null
    }

    let extra = 0
    if (type === 0xda) {
      if (sock.rQwait('JPEG', length - 2 + 2, 4)) {
        return null
      }

      let len = sock.rQlen()
      let data = sock.rQpeekBytes(len, false)

      while (true) {
        let idx = data.indexOf(0xff, length - 2 + extra)
        if (idx === -1) {
          sock.rQwait('JPEG', Infinity, 4)
          return null
        }

        if (idx === len - 1) {
          sock.rQwait('JPEG', Infinity, 4)
          return null
        }

        if (data.at(idx + 1) === 0x00 || (data.at(idx + 1) >= 0xd0 && data.at(idx + 1) <= 0xd7)) {
          extra = idx + 2 - (length - 2)
          continue
        }

        extra = idx - (length - 2)
        break
      }
    }

    let segment = new Uint8Array(2 + length + extra)
    segment[0] = marker
    segment[1] = type
    segment[2] = length >> 8
    segment[3] = length
    segment.set(sock.rQshiftBytes(length - 2 + extra, false), 4)

    return segment
  }
}

interface Header {
  _bytes4: number[]
  to_hex(): number[]
  to_binary16(encoder: unknown): number[]
  to_binary32(encoder: unknown): number[]
}
interface SendSession {
  _create_header_bytes(args: unknown[]): [number[], Header]
  _get_header_formatter(name: string): 'to_hex' | 'to_binary16' | 'to_binary32'
  _zencoder: unknown
}
export function setZmodemUploadPolicy(session: object, overwrite: boolean): void {
  const sender = session as SendSession
  const encode = sender._create_header_bytes.bind(sender)
  sender._create_header_bytes = (args) => {
    const result = encode(args)
    if (args[0] !== 'ZFILE') return result
    const header = result[1]
    header._bytes4[2] = overwrite ? 4 : 7
    return [header[sender._get_header_formatter('ZFILE')](sender._zencoder), header]
  }
}

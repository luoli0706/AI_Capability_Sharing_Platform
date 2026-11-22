function readU32LE(buf: Uint8Array, offset: number): number {
  return (
    (buf[offset] |
      (buf[offset + 1] << 8) |
      (buf[offset + 2] << 16) |
      (buf[offset + 3] << 24)) >>>
    0
  )
}

export function deobfuscateBytes(obf: Uint8Array): Uint8Array {
  if (obf.length < 16) {
    return obf
  }

  let s0 = (readU32LE(obf, 0) ^ 0x9e3779b9) >>> 0 || 1
  let s1 = (readU32LE(obf, 4) ^ 0x85ebca6b) >>> 0 || 1
  let s2 = (readU32LE(obf, 8) ^ 0xc2b2ae35) >>> 0 || 1
  let s3 = (readU32LE(obf, 12) ^ 0x27d4eb2f) >>> 0 || 1

  const payloadLen = obf.length - 16
  const out = new Uint8Array(payloadLen)
  let i = 0

  while (i + 4 <= payloadLen) {
    const t = (s0 ^ (s0 << 11)) >>> 0
    s0 = s1
    s1 = s2
    s2 = s3
    s3 = (s3 ^ (s3 >>> 19) ^ (t ^ (t >>> 8))) >>> 0

    out[i] = obf[16 + i] ^ (s3 & 0xff)
    out[i + 1] = obf[17 + i] ^ ((s3 >>> 8) & 0xff)
    out[i + 2] = obf[18 + i] ^ ((s3 >>> 16) & 0xff)
    out[i + 3] = obf[19 + i] ^ ((s3 >>> 24) & 0xff)
    i += 4
  }

  if (i < payloadLen) {
    const t = (s0 ^ (s0 << 11)) >>> 0
    s0 = s1
    s1 = s2
    s2 = s3
    s3 = (s3 ^ (s3 >>> 19) ^ (t ^ (t >>> 8))) >>> 0

    for (let k = 0; i + k < payloadLen; k++) {
      out[i + k] = obf[16 + i + k] ^ ((s3 >>> (k * 8)) & 0xff)
    }
  }

  return out
}

const utf8Decoder = new TextDecoder('utf-8')

export function deobfuscateBase64ToText(b64: string): string {
  if (!b64) {
    return ''
  }
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  const plain = deobfuscateBytes(bytes)
  return utf8Decoder.decode(plain)
}

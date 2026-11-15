/**
 * Minimal QR Code encoder (ISO/IEC 18004): byte mode, error correction level M, versions 1 to 40.
 * It is enough for invitation links and avoids a dependency.
 */

// Indexed by version; index 0 is unused.
const ECC_CODEWORDS_PER_BLOCK = [
  -1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28,
  28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28,
]
const ECC_BLOCKS = [
  -1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33,
  35, 37, 38, 40, 43, 45, 47, 49,
]
// Level M is 00 in the format information.
const ECC_FORMAT_BITS = 0

export interface QrCode {
  size: number
  /** modules[y][x] is true for a dark module. */
  modules: boolean[][]
}

function bit(value: number, i: number): boolean {
  return ((value >>> i) & 1) !== 0
}

function rawDataModules(version: number): number {
  let n = (16 * version + 128) * version + 64
  if (version >= 2) {
    const align = Math.floor(version / 7) + 2
    n -= (25 * align - 10) * align - 55
    if (version >= 7) n -= 36
  }
  return n
}

function dataCodewords(version: number): number {
  return Math.floor(rawDataModules(version) / 8) - ECC_CODEWORDS_PER_BLOCK[version]! * ECC_BLOCKS[version]!
}

function gfMultiply(x: number, y: number): number {
  let z = 0
  for (let i = 7; i >= 0; i--) {
    z = (z << 1) ^ ((z >>> 7) * 0x11d)
    z ^= ((y >>> i) & 1) * x
  }
  return z
}

/** Generator polynomial of the given degree, highest coefficient (always 1) omitted. */
export function rsDivisor(degree: number): number[] {
  const result = new Array<number>(degree).fill(0)
  result[degree - 1] = 1
  let root = 1
  for (let i = 0; i < degree; i++) {
    for (let j = 0; j < result.length; j++) {
      result[j] = gfMultiply(result[j]!, root)
      if (j + 1 < result.length) result[j]! ^= result[j + 1]!
    }
    root = gfMultiply(root, 0x02)
  }
  return result
}

/** Error correction codewords of a block. */
export function rsRemainder(data: readonly number[], divisor: readonly number[]): number[] {
  const result = divisor.map(() => 0)
  for (const b of data) {
    const factor = b ^ result.shift()!
    result.push(0)
    divisor.forEach((coef, i) => {
      result[i]! ^= gfMultiply(coef, factor)
    })
  }
  return result
}

/** 15-bit format information for level M and a mask. */
export function formatBits(mask: number): number {
  const data = (ECC_FORMAT_BITS << 3) | mask
  let rem = data
  for (let i = 0; i < 10; i++) rem = (rem << 1) ^ ((rem >>> 9) * 0x537)
  return ((data << 10) | rem) ^ 0x5412
}

/** 18-bit version information, used from version 7. */
export function versionBits(version: number): number {
  let rem = version
  for (let i = 0; i < 12; i++) rem = (rem << 1) ^ ((rem >>> 11) * 0x1f25)
  return (version << 12) | rem
}

function alignmentPositions(version: number): number[] {
  if (version === 1) return []
  const count = Math.floor(version / 7) + 2
  const size = version * 4 + 17
  const step = version === 32 ? 26 : Math.ceil((version * 4 + 4) / (count * 2 - 2)) * 2
  const result = [6]
  for (let pos = size - 7; result.length < count; pos -= step) result.splice(1, 0, pos)
  return result
}

function interleave(version: number, data: number[]): number[] {
  const blocks = ECC_BLOCKS[version]!
  const eccLen = ECC_CODEWORDS_PER_BLOCK[version]!
  const raw = Math.floor(rawDataModules(version) / 8)
  const shortBlocks = blocks - (raw % blocks)
  const shortLen = Math.floor(raw / blocks)
  const divisor = rsDivisor(eccLen)
  const all: number[][] = []
  for (let i = 0, k = 0; i < blocks; i++) {
    const dat = data.slice(k, k + shortLen - eccLen + (i < shortBlocks ? 0 : 1))
    k += dat.length
    const ecc = rsRemainder(dat, divisor)
    if (i < shortBlocks) dat.push(0)
    all.push(dat.concat(ecc))
  }
  const result: number[] = []
  for (let i = 0; i < all[0]!.length; i++) {
    all.forEach((block, j) => {
      if (i !== shortLen - eccLen || j >= shortBlocks) result.push(block[i]!)
    })
  }
  return result
}

function encodeData(bytes: Uint8Array): { version: number; codewords: number[] } {
  let version = 1
  for (; ; version++) {
    if (version > 40) throw new Error('data too long for a QR code')
    const countBits = version <= 9 ? 8 : 16
    if (4 + countBits + bytes.length * 8 <= dataCodewords(version) * 8) break
  }
  const capacity = dataCodewords(version) * 8
  const bits: number[] = []
  const push = (value: number, length: number) => {
    for (let i = length - 1; i >= 0; i--) bits.push((value >>> i) & 1)
  }
  push(0b0100, 4)
  push(bytes.length, version <= 9 ? 8 : 16)
  bytes.forEach((b) => push(b, 8))
  push(0, Math.min(4, capacity - bits.length))
  push(0, (8 - (bits.length % 8)) % 8)
  for (let pad = 0xec; bits.length < capacity; pad ^= 0xec ^ 0x11) push(pad, 8)
  const codewords: number[] = []
  for (let i = 0; i < bits.length; i += 8) codewords.push(bits.slice(i, i + 8).reduce((acc, b) => (acc << 1) | b, 0))
  return { version, codewords }
}

function masked(mask: number, x: number, y: number): boolean {
  switch (mask) {
    case 0:
      return (x + y) % 2 === 0
    case 1:
      return y % 2 === 0
    case 2:
      return x % 3 === 0
    case 3:
      return (x + y) % 3 === 0
    case 4:
      return (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0
    case 5:
      return ((x * y) % 2) + ((x * y) % 3) === 0
    case 6:
      return (((x * y) % 2) + ((x * y) % 3)) % 2 === 0
    default:
      return (((x + y) % 2) + ((x * y) % 3)) % 2 === 0
  }
}

/** Penalty score of a symbol (rules N1 to N4); the mask with the lowest one is kept. */
function penalty(m: boolean[][]): number {
  const size = m.length
  let score = 0
  const lines: boolean[][] = []
  for (let i = 0; i < size; i++) {
    lines.push(m[i]!)
    lines.push(m.map((row) => row[i]!))
  }
  const finderLike = [true, false, true, true, true, false, true]
  for (const line of lines) {
    let run = 1
    for (let i = 1; i <= size; i++) {
      if (i < size && line[i] === line[i - 1]) {
        run++
        continue
      }
      if (run >= 5) score += run - 2
      run = 1
    }
    for (let i = 0; i + 7 <= size; i++) {
      if (!finderLike.every((v, k) => line[i + k] === v)) continue
      const lightBefore = i >= 4 && [1, 2, 3, 4].every((k) => !line[i - k])
      const lightAfter = i + 11 <= size && [7, 8, 9, 10].every((k) => !line[i + k])
      if (lightBefore || lightAfter) score += 40
    }
  }
  let dark = 0
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      if (m[y]![x]) dark++
      if (x + 1 < size && y + 1 < size) {
        const c = m[y]![x]
        if (m[y]![x + 1] === c && m[y + 1]![x] === c && m[y + 1]![x + 1] === c) score += 3
      }
    }
  }
  const total = size * size
  score += Math.ceil(Math.abs(dark * 20 - total * 10) / total - 1) * 10
  return score
}

export function encodeQr(text: string): QrCode {
  const { version, codewords } = encodeData(new TextEncoder().encode(text))
  const size = version * 4 + 17
  const modules = Array.from({ length: size }, () => new Array<boolean>(size).fill(false))
  const fixed = Array.from({ length: size }, () => new Array<boolean>(size).fill(false))
  const set = (x: number, y: number, dark: boolean) => {
    modules[y]![x] = dark
    fixed[y]![x] = true
  }

  for (let i = 0; i < size; i++) {
    set(6, i, i % 2 === 0)
    set(i, 6, i % 2 === 0)
  }
  for (const [cx, cy] of [
    [3, 3],
    [size - 4, 3],
    [3, size - 4],
  ] as const) {
    for (let dy = -4; dy <= 4; dy++) {
      for (let dx = -4; dx <= 4; dx++) {
        const x = cx + dx
        const y = cy + dy
        const dist = Math.max(Math.abs(dx), Math.abs(dy))
        if (x >= 0 && x < size && y >= 0 && y < size) set(x, y, dist !== 2 && dist !== 4)
      }
    }
  }
  const align = alignmentPositions(version)
  const last = align.length - 1
  align.forEach((ay, i) =>
    align.forEach((ax, j) => {
      if ((i === 0 && j === 0) || (i === 0 && j === last) || (i === last && j === 0)) return
      for (let dy = -2; dy <= 2; dy++) {
        for (let dx = -2; dx <= 2; dx++) set(ax + dx, ay + dy, Math.max(Math.abs(dx), Math.abs(dy)) !== 1)
      }
    }),
  )
  const drawFormat = (mask: number) => {
    const bits = formatBits(mask)
    for (let i = 0; i <= 5; i++) set(8, i, bit(bits, i))
    set(8, 7, bit(bits, 6))
    set(8, 8, bit(bits, 7))
    set(7, 8, bit(bits, 8))
    for (let i = 9; i < 15; i++) set(14 - i, 8, bit(bits, i))
    for (let i = 0; i < 8; i++) set(size - 1 - i, 8, bit(bits, i))
    for (let i = 8; i < 15; i++) set(8, size - 15 + i, bit(bits, i))
    set(8, size - 8, true)
  }
  drawFormat(0)
  if (version >= 7) {
    const bits = versionBits(version)
    for (let i = 0; i < 18; i++) {
      const a = size - 11 + (i % 3)
      const b = Math.floor(i / 3)
      set(a, b, bit(bits, i))
      set(b, a, bit(bits, i))
    }
  }

  const data = interleave(version, codewords)
  let i = 0
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5
    for (let vert = 0; vert < size; vert++) {
      for (let j = 0; j < 2; j++) {
        const x = right - j
        const upward = ((right + 1) & 2) === 0
        const y = upward ? size - 1 - vert : vert
        if (!fixed[y]![x] && i < data.length * 8) {
          modules[y]![x] = bit(data[i >>> 3]!, 7 - (i & 7))
          i++
        }
      }
    }
  }

  const applyMask = (mask: number) => {
    for (let y = 0; y < size; y++) {
      for (let x = 0; x < size; x++) if (!fixed[y]![x] && masked(mask, x, y)) modules[y]![x] = !modules[y]![x]
    }
  }
  let best = 0
  let bestScore = Infinity
  for (let mask = 0; mask < 8; mask++) {
    applyMask(mask)
    drawFormat(mask)
    const score = penalty(modules)
    if (score < bestScore) {
      best = mask
      bestScore = score
    }
    applyMask(mask)
  }
  applyMask(best)
  drawFormat(best)
  return { size, modules }
}

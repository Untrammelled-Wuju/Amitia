const K = new Uint32Array([
  0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
  0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
  0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
  0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
  0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
]);

function rotr(value: number, count: number): number {
  return (value >>> count) | (value << (32 - count));
}

export class Sha256Stream {
  private state = new Uint32Array([0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]);
  private block = new Uint8Array(64);
  private blockLength = 0;
  private bytes = 0;

  update(input: Uint8Array): void {
    let offset = 0;
    this.bytes += input.length;
    if (this.blockLength > 0) {
      const needed = Math.min(64 - this.blockLength, input.length);
      this.block.set(input.subarray(0, needed), this.blockLength);
      this.blockLength += needed;
      offset += needed;
      if (this.blockLength === 64) {
        this.compress(this.block, 0);
        this.blockLength = 0;
      }
    }
    while (offset + 64 <= input.length) {
      this.compress(input, offset);
      offset += 64;
    }
    if (offset < input.length) {
      this.block.set(input.subarray(offset), 0);
      this.blockLength = input.length - offset;
    }
  }

  digestHex(): string {
    const totalBytes = this.bytes;
    this.block[this.blockLength++] = 0x80;
    if (this.blockLength > 56) {
      this.block.fill(0, this.blockLength);
      this.compress(this.block, 0);
      this.blockLength = 0;
    }
    this.block.fill(0, this.blockLength, 56);
    const bits = totalBytes * 8;
    const high = Math.floor(bits / 0x100000000);
    const low = bits >>> 0;
    this.block[56] = high >>> 24;
    this.block[57] = high >>> 16;
    this.block[58] = high >>> 8;
    this.block[59] = high;
    this.block[60] = low >>> 24;
    this.block[61] = low >>> 16;
    this.block[62] = low >>> 8;
    this.block[63] = low;
    this.compress(this.block, 0);
    return Array.from(this.state, (value) => value.toString(16).padStart(8, "0")).join("");
  }

  private compress(data: Uint8Array, offset: number): void {
    const w = new Uint32Array(64);
    for (let index = 0; index < 16; index++) {
      const position = offset + index * 4;
      w[index] = ((data[position] << 24) | (data[position + 1] << 16) | (data[position + 2] << 8) | data[position + 3]) >>> 0;
    }
    for (let index = 16; index < 64; index++) {
      const s0 = rotr(w[index - 15], 7) ^ rotr(w[index - 15], 18) ^ (w[index - 15] >>> 3);
      const s1 = rotr(w[index - 2], 17) ^ rotr(w[index - 2], 19) ^ (w[index - 2] >>> 10);
      w[index] = (w[index - 16] + s0 + w[index - 7] + s1) >>> 0;
    }
    let [a, b, c, d, e, f, g, h] = this.state;
    for (let index = 0; index < 64; index++) {
      const s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
      const choose = (e & f) ^ (~e & g);
      const temp1 = (h + s1 + choose + K[index] + w[index]) >>> 0;
      const s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
      const majority = (a & b) ^ (a & c) ^ (b & c);
      const temp2 = (s0 + majority) >>> 0;
      h = g;
      g = f;
      f = e;
      e = (d + temp1) >>> 0;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) >>> 0;
    }
    this.state[0] = (this.state[0] + a) >>> 0;
    this.state[1] = (this.state[1] + b) >>> 0;
    this.state[2] = (this.state[2] + c) >>> 0;
    this.state[3] = (this.state[3] + d) >>> 0;
    this.state[4] = (this.state[4] + e) >>> 0;
    this.state[5] = (this.state[5] + f) >>> 0;
    this.state[6] = (this.state[6] + g) >>> 0;
    this.state[7] = (this.state[7] + h) >>> 0;
  }
}

export async function sha256BlobHex(blob: Blob): Promise<string> {
  const hash = new Sha256Stream();
  const reader = blob.stream().getReader();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    hash.update(value);
  }
  return hash.digestHex();
}

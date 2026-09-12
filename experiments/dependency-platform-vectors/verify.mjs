// Independent Node/OpenSSL verifier: reads literal frozen data, never rewrites it.
// No PowerShell calculator or production Go package is loaded.
import fs from 'node:fs';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
const data = JSON.parse(fs.readFileSync(new URL('./vectors.json', import.meta.url), 'utf8'));
const hash = b => crypto.createHash('sha256').update(b).digest();
const hex = b => b.toString('hex');
const u = n => { const b = Buffer.alloc(8); b.writeBigUInt64BE(BigInt(n)); return b; };
const s = t => { const b = Buffer.from(t, 'utf8'); return Buffer.concat([u(b.length), b]); };
const frame = (...p) => Buffer.concat(p);
const uuid = b => { b = Buffer.from(b.subarray(0,16)); b[6] = (b[6]&15)|128; b[8]=(b[8]&63)|128; return hex(b).replace(/^(.{8})(.{4})(.{4})(.{4})(.{12})$/, '$1-$2-$3-$4-$5'); };
const profile = frame(...['dependency-platform-profile/v1','dependency-publication','0.1.0','dependency-inventory','0.1.0','dependency-canonical-json','0.1.0','application/json','dependency-platform-artifact-id/v1','dependency-platform-integration','0.1.0','dependency-platform-manifest/v1','logical-source-references-only'].map(s), u(1),u(0),u(0));
assert.equal(hex(profile),data.profile_preimage_hex);
assert.equal(hex(hash(profile)),data.profile_sha256);
let children=0;
for (const v of data.vectors) {
  const payload=Buffer.from(v.payload_base64,'base64');
  assert.deepEqual(payload,Buffer.from(v.payload_utf8,'utf8'));
  assert.equal(payload.length,v.payload_size);
  assert.equal(hex(hash(payload)),v.payload_sha256);
  assert.equal(payload.at(-1),10); assert.notEqual(payload.at(-2),10);
  assert(!payload.includes(13));
  JSON.parse(v.payload_utf8); // Syntax only: do not round uint64 through JS numbers.
  const id=frame(...['dependency-platform-artifact-id/v1',v.scope_id,v.repository_id,v.scan_id,'dependency-inventory','0.1.0'].map(s));
  assert.equal(hex(id),v.artifact_preimage_hex);
  assert.equal(hex(hash(id)),v.artifact_sha256);
  assert.equal(uuid(hash(id)),v.artifact_uuid);
  const manifest=frame(s('dependency-platform-manifest/v1'),s(v.scope_id),s(v.repository_id),s(v.scan_id),hash(profile),s(v.source_revision),u(1),...[v.artifact_uuid,'dependency-inventory','0.1.0','dependency-platform-artifact-id/v1','dependency-canonical-json','0.1.0','application/json','dependency-platform-integration','0.1.0'].map(s),hash(payload),u(payload.length),u(0),u(0),u(0));
  assert.equal(hex(manifest),v.manifest_preimage_hex);
  assert.equal(hex(hash(manifest)),v.manifest_sha256);
  for (const op of ['begin','stage','publish','fail','cancel']) {
    const p=frame(...['dependency-platform-request/v1',v.parent_request_id,op,v.scope_id,v.repository_id,v.scan_id].map(s),hash(manifest));
    assert.equal(hex(p),v.child_operations[op].preimage_hex);
    assert.equal(hex(hash(p)),v.child_operations[op].request_id); children++;
  }
  // Final LF and every raw manifest byte are integrity-sensitive.
  assert.notEqual(hex(hash(payload.subarray(0,-1))),v.payload_sha256);
  for(let i=0;i<manifest.length;i++) { const changed=Buffer.from(manifest); changed[i]^=1; assert.notEqual(hex(hash(changed)),v.manifest_sha256); }
}
for(const m of data.artifact_identity_mutations) {
  const p=frame(s('dependency-platform-artifact-id/v1'),...m.inputs.map(s));
  assert.equal(hex(p),m.preimage_hex); assert.equal(hex(hash(p)),m.sha256);
  assert.equal(uuid(hash(p)),m.uuid); assert.notEqual(m.uuid,data.vectors[0].artifact_uuid);
}
const empty=data.vectors.find(v=>v.name==='empty');
const unicode=data.vectors.find(v=>v.name==='unicode_omission');
for(const [v,expected] of [[empty,'27f88fcd44316dd1e7b7b93288fc9a98cb212d1f8cb82c315c537732c66060b2'],[unicode,'937f6b3fcad90715eae637ec428946ef628aaf7db5902b74d1f2730a76e951c1']]) {
  assert.equal(hex(hash(frame(s('dependency-analysis-input/v1'),Buffer.from(v.payload_base64,'base64')))),expected);
}
assert(data.vectors.find(v=>v.name==='uint64_wire_only').payload_utf8.includes('18446744073709551615'));
assert(data.vectors.find(v=>v.name==='uint64_wire_only').payload_utf8.includes('9007199254740993'));
console.log(`PASS: ${data.vectors.length} payload/profile/UUID/manifest vectors; ${children} child IDs; ${data.artifact_identity_mutations.length} identity mutations; every manifest-byte mutation; 2 existing digest anchors. Node ${process.version}, OpenSSL ${process.versions.openssl}.`);

import assert from 'node:assert';
import { AnsRotationResolver } from '../src/rotation_resolver.ts';
import { EndorsementGraph } from '../src/endorsement_graph.ts';

console.log('Testing AnsRotationResolver...');

const resolver = new AnsRotationResolver();

// Test 1: Register .acr name
resolver.registerName('alice.agent.acr', 'did:key:z6MkqAliceKey1');
assert.throws(() => resolver.registerName('invalid-name', 'did:key:z6MkqTest'), /must end with \.acr/);
console.log('[OK] Name registration and validation verified');

// Test 2: Unrotated resolution
const res1 = resolver.resolveName('alice.agent.acr');
assert.strictEqual(res1.currentKey, 'did:key:z6mkqalicekey1');
assert.strictEqual(res1.isRotated, false);
assert.strictEqual(res1.chainDepth, 0);
console.log('[OK] Unrotated name resolution verified');

// Test 3: Add single rotation link
const link1 = {
  predecessor: 'did:key:z6MkqAliceKey1',
  successor: 'did:key:z6MkqAliceKey2',
  timestamp: new Date().toISOString()
};
const added1 = resolver.addRotationLink(link1);
assert.strictEqual(added1, true);

const res2 = resolver.resolveName('alice.agent.acr');
assert.strictEqual(res2.currentKey, 'did:key:z6mkqalicekey2');
assert.strictEqual(res2.isRotated, true);
assert.strictEqual(res2.chainDepth, 1);
assert.deepStrictEqual(res2.history, ['did:key:z6mkqalicekey1', 'did:key:z6mkqalicekey2']);
console.log('[OK] 1-hop key rotation resolution verified');

// Test 4: Multi-hop key rotation chain
const link2 = {
  predecessor: 'did:key:z6MkqAliceKey2',
  successor: 'did:key:z6MkqAliceKey3',
  timestamp: new Date().toISOString()
};
resolver.addRotationLink(link2);

const res3 = resolver.resolveName('alice.agent.acr');
assert.strictEqual(res3.currentKey, 'did:key:z6mkqalicekey3');
assert.strictEqual(res3.isRotated, true);
assert.strictEqual(res3.chainDepth, 2);
assert.deepStrictEqual(res3.history, [
  'did:key:z6mkqalicekey1',
  'did:key:z6mkqalicekey2',
  'did:key:z6mkqalicekey3'
]);
console.log('[OK] 2-hop continuous key rotation chain verified');

// Test 5: Cycle guard prevents infinite loop
const cycleLink = {
  predecessor: 'did:key:z6MkqAliceKey3',
  successor: 'did:key:z6MkqAliceKey1',
  timestamp: new Date().toISOString()
};
resolver.addRotationLink(cycleLink);

const res4 = resolver.resolveName('alice.agent.acr');
assert.strictEqual(res4.currentKey, 'did:key:z6mkqalicekey3'); // Stops at cycle
console.log('[OK] Rotation link cycle guard verified');

// Test 6: Self-referential link rejection
const selfLink = {
  predecessor: 'did:key:z6MkqSelf',
  successor: 'did:key:z6MkqSelf',
  timestamp: new Date().toISOString()
};
assert.strictEqual(resolver.addRotationLink(selfLink), false);
console.log('[OK] Self-referential link rejection verified');

console.log('\nTesting EndorsementGraph (Web of Trust & BFS)...');

const graph = new EndorsementGraph();

// Node A endorses Node B, Node B endorses Node C, Node C endorses Node D
graph.addEndorsement({
  endorser: 'did:key:nodeA',
  endorsee: 'did:key:nodeB',
  timestamp: new Date().toISOString()
});
graph.addEndorsement({
  endorser: 'did:key:nodeB',
  endorsee: 'did:key:nodeC',
  timestamp: new Date().toISOString()
});
graph.addEndorsement({
  endorser: 'did:key:nodeC',
  endorsee: 'did:key:nodeD',
  timestamp: new Date().toISOString()
});

// Self-endorsement rejected
assert.strictEqual(graph.addEndorsement({
  endorser: 'did:key:nodeA',
  endorsee: 'did:key:nodeA',
  timestamp: new Date().toISOString()
}), false);

// Test BFS reachability with maxDepth=2
const reachableDepth2 = graph.trustedFrom(['did:key:nodeA'], 2);
assert.strictEqual(reachableDepth2.has('did:key:nodea'), true);
assert.strictEqual(reachableDepth2.get('did:key:nodea').distance, 0);
assert.strictEqual(reachableDepth2.has('did:key:nodeb'), true);
assert.strictEqual(reachableDepth2.get('did:key:nodeb').distance, 1);
assert.strictEqual(reachableDepth2.has('did:key:nodec'), true);
assert.strictEqual(reachableDepth2.get('did:key:nodec').distance, 2);
assert.strictEqual(reachableDepth2.has('did:key:noded'), false); // Exceeds maxDepth=2
console.log('[OK] Bounded BFS reachability verified');

// Test rotation-aware trust inheritance
// Suppose Node B rotates its key to Node B2
graph.registerRotation('did:key:nodeb', 'did:key:nodeb2');

// Direct check: Node B2 is not directly endorsed by Node A, but its predecessor was!
assert.strictEqual(graph.isTrustedVia(['did:key:nodeA'], 'did:key:nodeb2', 2), true);
assert.strictEqual(graph.isTrustedVia(['did:key:nodeA'], 'did:key:unknown', 2), false);
console.log('[OK] Web of Trust rotation inheritance verified');

console.log('\nALL ANS ROTATION RESOLVER & WEB OF TRUST TESTS PASSED (100% OK)');

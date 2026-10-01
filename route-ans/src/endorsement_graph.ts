/**
 * Agent Name Service (ANS) Transitive Web of Trust & Endorsement Graph
 *
 * Implements a directed trust graph with Ed25519 endorsements, bounded BFS traversal,
 * and key-rotation-aware trust inheritance along RotationLink chains.
 */

export interface EndorsementRecord {
  endorser: string; // DID or hex public key
  endorsee: string; // DID or hex public key
  role?: string;
  scope?: string;
  timestamp: string;
  signature?: string;
}

export interface TrustDistance {
  target: string;
  distance: number;
  path: string[];
}

export class EndorsementGraph {
  // endorser -> set of endorsees
  private readonly adjacency: Map<string, Set<string>> = new Map();
  // successor -> predecessor (reverse map of key rotations)
  private readonly predecessorMap: Map<string, string> = new Map();

  /**
   * Adds a verified endorsement edge from endorser to endorsee.
   */
  public addEndorsement(record: EndorsementRecord): boolean {
    const from = record.endorser.toLowerCase().trim();
    const to = record.endorsee.toLowerCase().trim();

    if (from === to) {
      return false; // Prevent self-endorsement
    }

    let edges = this.adjacency.get(from);
    if (!edges) {
      edges = new Set<string>();
      this.adjacency.set(from, edges);
    }
    edges.add(to);
    return true;
  }

  /**
   * Registers a verified key rotation link (predecessor -> successor).
   */
  public registerRotation(predecessor: string, successor: string): void {
    const pred = predecessor.toLowerCase().trim();
    const succ = successor.toLowerCase().trim();
    if (pred !== succ) {
      this.predecessorMap.set(succ, pred);
    }
  }

  /**
   * Computes all reachable peers from a set of root trust anchors within maxDepth steps using BFS.
   */
  public trustedFrom(roots: string[], maxDepth = 3): Map<string, TrustDistance> {
    const visited = new Map<string, TrustDistance>();
    const queue: Array<{ key: string; depth: number; path: string[] }> = [];

    for (const r of roots) {
      const rootKey = r.toLowerCase().trim();
      visited.set(rootKey, { target: rootKey, distance: 0, path: [rootKey] });
      queue.push({ key: rootKey, depth: 0, path: [rootKey] });
    }

    while (queue.length > 0) {
      const current = queue.shift()!;
      if (current.depth >= maxDepth) {
        continue;
      }

      const neighbors = this.adjacency.get(current.key);
      if (!neighbors) {
        continue;
      }

      for (const neighbor of neighbors) {
        if (!visited.has(neighbor)) {
          const newPath = [...current.path, neighbor];
          const dist: TrustDistance = {
            target: neighbor,
            distance: current.depth + 1,
            path: newPath
          };
          visited.set(neighbor, dist);
          queue.push({ key: neighbor, depth: current.depth + 1, path: newPath });
        }
      }
    }

    return visited;
  }

  /**
   * Checks whether a target key is trusted from roots within maxDepth,
   * taking into account key rotation chains if target has been rotated.
   */
  public isTrustedVia(roots: string[], target: string, maxDepth = 3): boolean {
    const reachable = this.trustedFrom(roots, maxDepth);
    const normalizedTarget = target.toLowerCase().trim();

    // Check direct reachability
    if (reachable.has(normalizedTarget)) {
      return true;
    }

    // Traverse predecessors along rotation chain to check if older key was trusted
    let current = normalizedTarget;
    const seen = new Set<string>([current]);

    while (this.predecessorMap.has(current)) {
      const pred = this.predecessorMap.get(current)!;
      if (seen.has(pred)) {
        break; // Cycle guard
      }
      seen.add(pred);

      if (reachable.has(pred)) {
        return true; // Trust inherited from rotated predecessor!
      }
      current = pred;
    }

    return false;
  }
}

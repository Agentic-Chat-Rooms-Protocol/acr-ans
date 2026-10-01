/**
 * Agent Name Service (ANS) Key Rotation Chain Resolver
 *
 * Resolves .acr names and DIDs by traversing dual-signed RotationLink records.
 * Ensures reputation, credentials, and trust standing are preserved across
 * continuous key migrations while preventing circular resolution loops.
 */

export interface RotationLinkRecord {
  predecessor: string;
  successor: string;
  timestamp: string;
  predecessor_sig?: string;
  successor_sig?: string;
}

export interface AnsResolutionResult {
  name?: string;
  initialKey: string;
  currentKey: string;
  chainDepth: number;
  history: string[];
  isRotated: boolean;
}

export class AnsRotationResolver {
  private readonly nameRegistry: Map<string, string> = new Map(); // name -> initialKey
  private readonly rotationEdges: Map<string, string> = new Map(); // predecessor -> successor
  private readonly maxDepth: number;

  constructor(maxDepth = 64) {
    this.maxDepth = maxDepth;
  }

  /**
   * Registers a .acr name to an initial public key or DID.
   */
  public registerName(name: string, initialKey: string): void {
    const normalized = name.toLowerCase().trim();
    if (!normalized.endsWith('.acr')) {
      throw new Error(`Invalid ANS name: must end with .acr, got "${name}"`);
    }
    this.nameRegistry.set(normalized, initialKey.toLowerCase().trim());
  }

  /**
   * Adds a verified RotationLink to the resolver chain. First valid link per predecessor wins.
   */
  public addRotationLink(link: RotationLinkRecord): boolean {
    const pred = link.predecessor.toLowerCase().trim();
    const succ = link.successor.toLowerCase().trim();

    if (pred === succ) {
      return false; // Reject self-referential links
    }

    if (!this.rotationEdges.has(pred)) {
      this.rotationEdges.set(pred, succ);
      return true;
    }

    return false;
  }

  /**
   * Resolves an arbitrary key or DID to its active successor key.
   */
  public resolveKey(startKey: string): AnsResolutionResult {
    const normalized = startKey.toLowerCase().trim();
    let curr = normalized;
    const history: string[] = [curr];
    const seen = new Set<string>([curr]);

    for (let depth = 0; depth < this.maxDepth; depth++) {
      const nextKey = this.rotationEdges.get(curr);
      if (!nextKey) {
        break; // Reached terminal active key
      }
      if (seen.has(nextKey)) {
        break; // Cycle guard: break out to prevent infinite loop
      }
      seen.add(nextKey);
      history.push(nextKey);
      curr = nextKey;
    }

    return {
      initialKey: normalized,
      currentKey: curr,
      chainDepth: history.length - 1,
      history,
      isRotated: curr !== normalized
    };
  }

  /**
   * Resolves a .acr name to its active canonical key/DID.
   */
  public resolveName(name: string): AnsResolutionResult | null {
    const normalized = name.toLowerCase().trim();
    const initialKey = this.nameRegistry.get(normalized);
    if (!initialKey) {
      return null;
    }

    const res = this.resolveKey(initialKey);
    return {
      name: normalized,
      ...res
    };
  }
}

import type { DiagramItem } from "./element-model.ts";
import type { ElementOperation } from "./element-operations.ts";
import { max, min } from "./layout-base.ts";
import type { LayoutAlgorithm, LayoutContext, Rect } from "./layout-base.ts";

/** POSIX drand48 with the default seed, which is what Swift's unseeded drand48() yields in a fresh process. */
export function makeDrand48(seed = 0x1234abcd330en): () => number {
  let x = seed;
  return () => {
    x = (0x5deece66dn * x + 0xbn) & 0xffffffffffffn;
    return Number(x) / 2 ** 48;
  };
}

export class SpringLayout implements LayoutAlgorithm {
  /** The default value for the spring layout number of iterations. */
  readonly DEFAULT_SPRING_ITERATIONS = 1000;
  /** The default value for the time algorithm runs. */
  readonly MAX_SPRING_TIME = 1;
  /** The default value for positioning nodes randomly. */
  readonly DEFAULT_SPRING_RANDOM = false;
  readonly DEFAULT_SPRING_MOVE = 1.0;
  readonly DEFAULT_SPRING_STRAIN = 1.0;
  readonly DEFAULT_SPRING_LENGTH = 3.0;
  readonly DEFAULT_SPRING_GRAVITATION = 2;
  /** Minimum distance considered between nodes */
  readonly MIN_DISTANCE = 50.0;

  sprIterations: number;
  /** Swift divides elapsed seconds by this, so despite the name it is seconds. */
  maxTimeMS: number;
  sprRandom: boolean;
  sprMove: number;
  sprStrain: number;
  sprLength: number;
  sprGravitation: number;

  resize = false;

  iteration = 0;
  srcDestToSumOfWeights: number[][] = [];
  entities: DiagramItem[] = [];
  forcesX: number[] = [];
  forcesY: number[] = [];
  locationsX: number[] = [];
  locationsY: number[] = [];
  sizeW: number[] = [];
  sizeH: number[] = [];
  bounds: Rect = { x: 0, y: 0, width: 0, height: 0 };
  boundsScaleX = 1;
  boundsScaleY = 1;

  layoutContext!: LayoutContext;

  fitWithinBoundsEnabled = true; // Swift: the `fitWithinBounds` var, which shares its name with the method.

  operations: ElementOperation[] = [];

  startTime = 0;

  // Seconds clock and random source are injectable so runs are reproducible.
  now: () => number;
  random: () => number;

  constructor(now: () => number = () => Date.now() / 1000, random: () => number = makeDrand48()) {
    this.now = now;
    this.random = random;
    this.sprIterations = this.DEFAULT_SPRING_ITERATIONS;
    this.maxTimeMS = this.MAX_SPRING_TIME;
    this.sprRandom = this.DEFAULT_SPRING_RANDOM;
    this.sprMove = this.DEFAULT_SPRING_MOVE;
    this.sprStrain = this.DEFAULT_SPRING_STRAIN;
    this.sprLength = this.DEFAULT_SPRING_LENGTH;
    this.sprGravitation = this.DEFAULT_SPRING_GRAVITATION;
  }

  apply(context: LayoutContext, clean: boolean): ElementOperation[] {
    this.layoutContext = context;
    this.initLayout(context);
    if (!clean) {
      return [];
    }

    while (this.performAnotherNonContinuousIteration()) {
      this.computeOneIteration();
    }

    this.saveLocations();

    if (this.fitWithinBoundsEnabled) {
      const insets = 4;
      const bounds2: Rect = {
        x: this.bounds.x + insets,
        y: this.bounds.y + insets,
        width: this.bounds.width - 2 * insets,
        height: this.bounds.height - 2 * insets,
      };
      this.fitWithinBounds(bounds2);
      this.saveLocations();
    }
    return this.operations;
  }

  fitWithinBounds(destinationBounds: Rect): void {
    if (this.entities.length === 1) {
      return;
    }
    const startingBounds = this.getLayoutBounds();
    for (let i = 0; i < this.entities.length; i++) {
      const entity = this.entities[i]!;
      const size = this.layoutContext.getBounds(entity);
      if (this.layoutContext.isMovable(entity)) {
        const locationX = this.locationsX[i]!;
        const locationY = this.locationsY[i]!;
        const percentX = startingBounds.width === 0 ? 0 : (locationX - startingBounds.x) / startingBounds.width;
        const percentY = startingBounds.height === 0 ? 0 : (locationY - startingBounds.y) / startingBounds.height;
        this.locationsX[i] = destinationBounds.x + size.width / 2 + percentX * (destinationBounds.width - size.width);
        this.locationsY[i] = destinationBounds.y + size.height / 2 + percentY * (destinationBounds.height - size.height);
      }
    }
  }

  private initLayout(context: LayoutContext): void {
    this.entities = context.nodes;
    this.bounds = context.getViewBounds();
    this.loadLocations();

    const n = this.entities.length;
    this.srcDestToSumOfWeights = Array.from({ length: n }, () => new Array<number>(n).fill(0));
    const entityToPosition = new Map<DiagramItem, number>();
    for (let i = 0; i < n; i++) {
      entityToPosition.set(this.entities[i]!, i);
    }

    for (const connection of context.edges) {
      if (connection.source === null || connection.target === null) {
        continue;
      }
      const source = entityToPosition.get(connection.source);
      const target = entityToPosition.get(connection.target);
      if (source !== undefined && target !== undefined) {
        let weight = context.getWeight(connection);
        weight = weight <= 0 ? 0.1 : weight;
        this.srcDestToSumOfWeights[source]![target]! += weight;
        this.srcDestToSumOfWeights[target]![source]! += weight;
      }
    }

    if (this.sprRandom) {
      this.placeRandomly(); // put vertices in random places
    }

    this.iteration = 1;

    this.startTime = this.now();
  }

  private loadLocations(): void {
    if (this.locationsX.length !== this.entities.length) {
      const length = this.entities.length;
      this.locationsX = new Array<number>(length).fill(0);
      this.locationsY = new Array<number>(length).fill(0);
      this.sizeW = new Array<number>(length).fill(0);
      this.sizeH = new Array<number>(length).fill(0);
      this.forcesX = new Array<number>(length).fill(0);
      this.forcesY = new Array<number>(length).fill(0);
    }
    for (let i = 0; i < this.entities.length; i++) {
      const ent = this.entities[i]!;
      this.locationsX[i] = ent.x;
      this.locationsY[i] = ent.y;
      const size = this.layoutContext.getBounds(ent);
      this.sizeW[i] = size.width;
      this.sizeH[i] = size.height;
    }
  }

  private saveLocations(): void {
    if (this.entities.length === 0) {
      return;
    }
    this.operations = [];
    for (let i = 0; i < this.entities.length; i++) {
      // NaN positions are reset to the origin (Swift TODO: find where they come from).
      if (Number.isNaN(this.locationsX[i]) || Number.isNaN(this.locationsY[i])) {
        this.locationsX[i] = 0;
        this.locationsY[i] = 0;
      }
      this.operations.push(
        this.layoutContext.store.createUpdatePosition(this.entities[i]!, { x: this.locationsX[i]!, y: this.locationsY[i]! }),
      );
    }
  }

  /** Scales the iteration counter by wall time: after maxTimeMS (seconds) the algorithm jumps to the last iteration. */
  private setSprIterationsBasedOnTime(): void {
    if (this.maxTimeMS <= 0) {
      return;
    }

    const since = this.now() - this.startTime;
    const fractionComplete = since / this.maxTimeMS;
    const currentIteration = Math.trunc(fractionComplete * this.sprIterations);
    if (currentIteration > this.iteration) {
      this.iteration = currentIteration;
    }
  }

  performAnotherNonContinuousIteration(): boolean {
    this.setSprIterationsBasedOnTime();
    return this.iteration <= this.sprIterations;
  }

  computeOneIteration(): void {
    this.computeForces();
    this.computePositions();
    const currentBounds = this.getLayoutBounds();
    this.improveBoundScaleX(currentBounds);
    this.improveBoundScaleY(currentBounds);
    this.moveToCenter(currentBounds);
    this.iteration += 1;
  }

  /** Puts vertices in random places inside the view bounds. */
  placeRandomly(): void {
    if (this.locationsX.length === 0) {
      return;
    }

    // If only one node in the data repository, put it in the middle
    if (this.locationsX.length === 1) {
      this.locationsX[0] = this.bounds.x + 0.5 * this.bounds.width;
      this.locationsY[0] = this.bounds.y + 0.5 * this.bounds.height;
    } else {
      this.locationsX[0] = this.bounds.x;
      this.locationsY[0] = this.bounds.y;
      this.locationsX[1] = this.bounds.x + this.bounds.width;
      this.locationsY[1] = this.bounds.y + this.bounds.height;
      for (let i = 2; i < this.locationsX.length; i++) {
        this.locationsX[i] = this.bounds.x + this.random() * this.bounds.width;
        this.locationsY[i] = this.bounds.y + this.random() * this.bounds.height;
      }
    }
  }

  /** Computes the force for each node. Two passes: the second starts from positions moved by the first. */
  computeForces(): void {
    const count = this.forcesX.length;
    const forcesX: number[][] = [new Array<number>(count).fill(0), new Array<number>(count).fill(0)];
    const forcesY: number[][] = [new Array<number>(count).fill(0), new Array<number>(count).fill(0)];
    const locationsX: number[] = new Array<number>(count).fill(0);
    const locationsY: number[] = new Array<number>(this.forcesY.length).fill(0);

    for (let i = 0; i < count; i++) {
      locationsX[i] = this.locationsX[i]!;
      locationsY[i] = this.locationsY[i]!;
    }

    for (let k = 0; k < 2; k++) {
      const fx = forcesX[k]!;
      const fy = forcesY[k]!;
      for (let i = 0; i < this.locationsX.length; i++) {
        for (let j = i + 1; j < this.locationsX.length; j++) {
          // Avoid division by zero when bounds width/height are zero.
          const bw = max(this.bounds.width, 1e-6);
          const bh = max(this.bounds.height, 1e-6);
          const dx = (locationsX[i]! - locationsX[j]!) / bw / this.boundsScaleX;
          const dy = (locationsY[i]! - locationsY[j]!) / bh / this.boundsScaleY;
          let distance_sq = dx * dx + dy * dy;
          // make sure distance and distance squared not too small
          distance_sq = max(this.MIN_DISTANCE * this.MIN_DISTANCE, distance_sq);
          const distance = Math.sqrt(distance_sq);

          // If there are relationships between srcObj and destObj then decrease force on srcObj (a pull) in direction of destObj.
          // If no relation between srcObj and destObj then increase force on srcObj (a push) from direction of destObj.
          const sumOfWeights = this.srcDestToSumOfWeights[i]![j]!;

          let f: number;
          if (sumOfWeights > 0) {
            // nodes are pulled towards each other
            f = -this.sprStrain * Math.log(distance / this.sprLength) * sumOfWeights;
          } else {
            // nodes are repelled from each other
            f = this.sprGravitation / distance_sq;
          }
          const dfx = (f * dx) / distance;
          const dfy = (f * dy) / distance;

          fx[i]! += dfx;
          fy[i]! += dfy;

          fx[j]! -= dfx;
          fy[j]! -= dfy;
        }
      }

      for (let i = 0; i < this.entities.length; i++) {
        if (this.layoutContext.isMovable(this.entities[i]!)) {
          let deltaX = this.sprMove * fx[i]!;
          let deltaY = this.sprMove * fy[i]!;

          // constrain movement, so that nodes don't shoot way off to the edge
          const dist = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
          const maxMovement = 0.2 * this.sprMove;
          if (dist > maxMovement) {
            deltaX = (deltaX * maxMovement) / dist;
            deltaY = (deltaY * maxMovement) / dist;
          }

          locationsX[i]! += deltaX * this.bounds.width * this.boundsScaleX;
          locationsY[i]! += deltaY * this.bounds.height * this.boundsScaleY;
        }
      }
    }
    for (let i = 0; i < this.entities.length; i++) {
      this.forcesX[i] = forcesX[0]![i]! * forcesX[1]![i]! < 0 ? 0 : forcesX[1]![i]!;
      this.forcesY[i] = forcesY[0]![i]! * forcesY[1]![i]! < 0 ? 0 : forcesY[1]![i]!;
    }
  }

  /** position = position + sprMove * force */
  computePositions(): void {
    for (let i = 0; i < this.entities.length; i++) {
      if (this.layoutContext.isMovable(this.entities[i]!)) {
        let deltaX = this.sprMove * this.forcesX[i]!;
        let deltaY = this.sprMove * this.forcesY[i]!;

        // constrain movement, so that nodes don't shoot way off to the edge
        const dist = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
        const maxMovement = 0.2 * this.sprMove;
        if (dist > maxMovement) {
          deltaX = (deltaX * maxMovement) / dist;
          deltaY = (deltaY * maxMovement) / dist;
        }

        this.locationsX[i]! += deltaX * this.bounds.width * this.boundsScaleX;
        this.locationsY[i]! += deltaY * this.bounds.height * this.boundsScaleY;
      }
    }
  }

  private getLayoutBounds(): Rect {
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -1 * Infinity;
    let maxY = -1 * Infinity;

    for (let i = 0; i < this.locationsX.length; i++) {
      maxX = max(maxX, this.locationsX[i]! + this.sizeW[i]!);
      minX = min(minX, this.locationsX[i]! - this.sizeW[i]!);
      maxY = max(maxY, this.locationsY[i]! + this.sizeH[i]!);
      minY = min(minY, this.locationsY[i]! - this.sizeH[i]!);
    }
    return { x: minX, y: minY, width: maxX - minX, height: maxY - minY };
  }

  private improveBoundScaleX(currentBounds: Rect): void {
    const boundaryProportionX = currentBounds.width / this.bounds.width;
    if (boundaryProportionX < 0.9) {
      this.boundsScaleX *= 1.01;
    } else if (boundaryProportionX > 1) {
      if (this.boundsScaleX < 0.01) {
        return;
      }
      this.boundsScaleX /= 1.01;
    }
  }

  private improveBoundScaleY(currentBounds: Rect): void {
    const boundaryProportionY = currentBounds.height / this.bounds.height;
    if (boundaryProportionY < 0.9) {
      this.boundsScaleY *= 1.01;
    } else if (boundaryProportionY > 1) {
      if (this.boundsScaleY < 0.01) {
        return;
      }
      this.boundsScaleY /= 1.01;
    }
  }

  private moveToCenter(currentBounds: Rect): void {
    const moveX = currentBounds.x + currentBounds.width / 2 - (this.bounds.x + this.bounds.width / 2);
    const moveY = currentBounds.y + currentBounds.height / 2 - (this.bounds.y + this.bounds.height / 2);
    for (let i = 0; i < this.locationsX.length; i++) {
      this.locationsX[i]! -= moveX;
      this.locationsY[i]! -= moveY;
    }
  }
}

//
//  SpringLayout.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 13.12.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation

class SpringLayout: LayoutAlgorithm {
    /**
     * The default value for the spring layout number of iterations.
     */
    let DEFAULT_SPRING_ITERATIONS = 1000;

    /**
     * the default value for the time algorithm runs.
     */
    let MAX_SPRING_TIME = 1;

    /**
     * The default value for positioning nodes randomly.
     */
    let DEFAULT_SPRING_RANDOM = false;

    /**
     * The default value for the spring layout move-control.
     */
    let DEFAULT_SPRING_MOVE = CGFloat(1.0);

    /**
     * The default value for the spring layout strain-control.
     */
    let DEFAULT_SPRING_STRAIN = CGFloat(1.0);

    /**
     * The default value for the spring layout length-control.
     */
    let DEFAULT_SPRING_LENGTH = CGFloat(3.0);

    /**
     * The default value for the spring layout gravitation-control.
     */
    let DEFAULT_SPRING_GRAVITATION = CGFloat(2);

    /**
     * Minimum distance considered between nodes
     */
    let MIN_DISTANCE = CGFloat(50.0);

    /**
     * The variable can be customized to set the number of iterations used.
     */
    var sprIterations: Int

    /**
     * This variable can be customized to set the max number of MS the algorithm
     * should run
     */
    var maxTimeMS: Int

    /**
     * The variable can be customized to set whether or not the spring layout
     * nodes are positioned randomly before beginning iterations.
     */
    var sprRandom: Bool

    /**
     * The variable can be customized to set the spring layout move-control.
     */
    var sprMove: CGFloat

    /**
     * The variable can be customized to set the spring layout strain-control.
     */
    var sprStrain: CGFloat

    /**
     * The variable can be customized to set the spring layout length-control.
     */
    var sprLength: CGFloat

    /**
     * The variable can be customized to set the spring layout
     * gravitation-control.
     */
    var sprGravitation: CGFloat

    /**
     * Variable indicating whether the algorithm should resize elements.
     */
    var resize = false;

    var iteration: Int = 0;
    var srcDestToSumOfWeights: [[CGFloat]] = [];
    var entities: [DiagramItem] = [];
    var forcesX:[CGFloat] = []
    var forcesY:[CGFloat] = [];
    var locationsX: [CGFloat] = []
    var locationsY: [CGFloat] = [];
    var sizeW: [CGFloat] = []
    var sizeH: [CGFloat] = [];
    var bounds: CGRect = CGRect();
    var boundsScaleX = CGFloat(1);
    var boundsScaleY = CGFloat(1);

    // XXX: Needed by performNIteration(int), see below.
    var layoutContext: LayoutContext!

    // TODO: expose field
    var fitWithinBounds = true;
    
    var operations: [ElementOperation] = []
    
    init() {
        sprIterations = DEFAULT_SPRING_ITERATIONS;
        maxTimeMS = MAX_SPRING_TIME;
        sprRandom = DEFAULT_SPRING_RANDOM;
        sprMove = DEFAULT_SPRING_MOVE;
        sprStrain = DEFAULT_SPRING_STRAIN;
        sprLength = DEFAULT_SPRING_LENGTH;
        sprGravitation = DEFAULT_SPRING_GRAVITATION;
    }

    func apply(context: LayoutContext, clean: Bool) -> [ElementOperation] {
        self.layoutContext = context;
        self.initLayout(context);
        if !clean {
            return [];
        }

        while self.performAnotherNonContinuousIteration() {
            self.computeOneIteration();
        }

        self.saveLocations();
        if (resize) {
//            AlgorithmHelper.maximizeSizes(entities);
        }

        if self.fitWithinBounds {
            var bounds2 = CGRect(origin: bounds.origin, size: bounds.size)
            let insets = CGFloat(4);
            bounds2.origin.x += insets;
            bounds2.origin.y += insets;
            bounds2.size.width -= 2 * insets;
            bounds2.size.height -= 2 * insets;
            self.fitWithinBounds(bounds2);
            self.saveLocations();
        }
        return self.operations;
    }
    
    private func getLayoutBounds(_ includeNodeSize:Bool ) -> CGRect {
        var rightSide = -1 * CGFloat.infinity;
        var bottomSide = -1 * CGFloat.infinity;
        var leftSide = CGFloat.infinity;
        var topSide = CGFloat.infinity;
        for i in 0..<self.entities.count {
            let entity = entities[i];
            let locationX = self.locationsX[i]
            let locationY = self.locationsY[i]
            let size = self.layoutContext.getBounds(node: entity);
            if (includeNodeSize) {
                leftSide = min(locationX - size.width / 2, leftSide);
                topSide = min(locationY - size.height / 2, topSide);
                rightSide = max(locationX + size.width / 2, rightSide);
                bottomSide = max(locationY + size.height / 2, bottomSide);
            } else {
                leftSide = min(locationX, leftSide);
                topSide = min(locationY, topSide);
                rightSide = max(locationX, rightSide);
                bottomSide = max(locationY, bottomSide);
            }
        }
        return CGRect(x: leftSide, y: topSide, width: rightSide - leftSide, height: bottomSide - topSide);
    }
    
    func fitWithinBounds(_ destinationBounds: CGRect) {
        if (self.entities.count == 1) {
            return;
        }
        let startingBounds = getLayoutBounds();
        let sizeScale = min(
                destinationBounds.width / startingBounds.width,
                destinationBounds.height / startingBounds.height);
        for i in 0..<self.entities.count{
            let entity = entities[i];
            let size = self.layoutContext.getBounds(node: entity);
            if (self.layoutContext.isMovable(entity)) {
                let locationX = self.locationsX[i]
                let locationY = self.locationsY[i]
                let percentX = startingBounds.width == 0 ? 0
                    : (locationX - startingBounds.origin.x)
                                / (startingBounds.width);
                let percentY = startingBounds.height == 0 ? 0
                    : (locationY - startingBounds.origin.y)
                                / (startingBounds.height);
                self.locationsX[i] = destinationBounds.origin.x + size.width / 2 + percentX * (destinationBounds.width - size.width);
                self.locationsY[i] = destinationBounds.origin.y + size.height / 2 + percentY * (destinationBounds.height - size.height);
            }
        }
    }

    /**
     * Performs the given number of iterations.
     *
     * @param n
     *            The number of iterations to perform.
     */
    func performNIteration(_ n: Int) {
        layoutContext.preLayout();
        if (iteration == 0) {
            entities = self.layoutContext.nodes
            self.loadLocations();
            self.initLayout(layoutContext);
        }
        bounds = layoutContext.getViewBounds()
        for _ in 0..<n {
            self.computeOneIteration();
            self.saveLocations();
        }
        layoutContext.postLayout();
    }

    /**
     * Performs one single iteration.
     *
     */
    func performOneIteration() {
        layoutContext.preLayout();
        if iteration == 0 {
            entities = layoutContext.nodes
            self.loadLocations();
            self.initLayout(layoutContext);
        }
        bounds = layoutContext.getBounds()
        self.computeOneIteration();
        self.saveLocations();
        layoutContext.postLayout();
    }

    /**
     *
     * @return true if this algorithm is set to resize elements
     */
    func isResizing() -> Bool {
        return self.resize;
    }

    /**
     *
     * @param resizing
     *            true if this algorithm should resize elements (default is
     *            false)
     */
    public func setResizing(_ resizing: Bool) {
        resize = resizing;
    }

    /**
     * Sets the spring layout move-control.
     *
     * @param move
     *            The move-control value.
     */
    public func setSpringMove(_ move: CGFloat) {
        sprMove = move;
    }

    /**
     * Returns the move-control value of this SpringLayoutAlgorithm in double
     * precision.
     *
     * @return The move-control value.
     */
    public func getSpringMove() -> CGFloat {
        return sprMove;
    }

    /**
     * Sets the spring layout strain-control.
     *
     * @param strain
     *            The strain-control value.
     */
    public func setSpringStrain(_ strain: CGFloat) {
        sprStrain = strain;
    }

    /**
     * Returns the strain-control value of this SpringLayoutAlgorithm in double
     * precision.
     *
     * @return The strain-control value.
     */
    public func getSpringStrain() -> CGFloat {
        return sprStrain;
    }

    /**
     * Sets the spring layout length-control.
     *
     * @param length
     *            The length-control value.
     */
    public func setSpringLength(_ length: CGFloat) {
        sprLength = length;
    }

    /**
     * Gets the max time this algorithm will run for
     *
     * @return the timeout up to which this algorithm may run
     */
    public func getSpringTimeout()-> Int {
        return maxTimeMS;
    }

    /**
     * Sets the spring timeout to the given value (in millis).
     *
     * @param timeout
     *            The new spring timeout (in millis).
     */
    public func setSpringTimeout(_ timeout: Int) {
        maxTimeMS = timeout;
    }

    /**
     * Returns the length-control value of this {@link SpringLayoutAlgorithm} in
     * double precision.
     *
     * @return The length-control value.
     */
    public func getSpringLength() -> CGFloat {
        return sprLength;
    }

    /**
     * Sets the spring layout gravitation-control.
     *
     * @param gravitation
     *            The gravitation-control value.
     */
    public func setSpringGravitation(_ gravitation: CGFloat) {
        sprGravitation = gravitation;
    }

    /**
     * Returns the gravitation-control value of this SpringLayoutAlgorithm in
     * double precision.
     *
     * @return The gravitation-control value.
     */
    public func getSpringGravitation() -> CGFloat {
        return sprGravitation;
    }

    /**
     * Sets the number of iterations to be used.
     *
     * @param iterations
     *            The number of iterations.
     */
    public func setIterations(_ iterations: Int) {
        sprIterations = iterations;
    }

    /**
     * Returns the number of iterations to be used.
     *
     * @return The number of iterations.
     */
    public func getIterations() -> Int {
        return sprIterations;
    }

    /**
     * Sets whether or not this SpringLayoutAlgorithm will layout the nodes
     * randomly before beginning iterations.
     *
     * @param random
     *            The random placement value.
     */
    public func setRandom(_ random: Bool ) {
        sprRandom = random;
    }

    /**
     * Returns whether or not this {@link SpringLayoutAlgorithm} will layout the
     * nodes randomly before beginning iterations.
     *
     * @return <code>true</code> if this algorithm will layout the nodes
     *         randomly before iterating, otherwise <code>false</code>.
     */
    public func getRandom() -> Bool {
        return sprRandom;
    }

    var startTime: Date = Date()

    private func initLayout(_ context: LayoutContext) {
        entities = context.nodes
        bounds = context.getViewBounds()
        self.loadLocations();

        srcDestToSumOfWeights = Array(repeating: Array(repeating: 0, count: entities.count), count: entities.count)
        var entityToPosition: [DiagramItem: Int] = [:]
        for i in 0..<entities.count {
            entityToPosition[entities[i]] =  i
        }

        let connections = context.edges
        for i in 0..<connections.count {
            let connection = connections[i];
            if connection.source == nil || connection.target == nil {
                continue
            }
            if let source = entityToPosition[connection.source!], let target = entityToPosition[connection.target!] {
                var weight = context.getWeight(connection)
                weight = (weight <= 0 ? 0.1 : weight);
                srcDestToSumOfWeights[source][target] += weight;
                srcDestToSumOfWeights[target][source] += weight;
            }
        }

        if sprRandom {
            self.placeRandomly(); // put vertices in random places
        }

        iteration = 1;

        startTime = Date()
    }

    private func loadLocations() {
        if (locationsX.count != entities.count) {
            let length = entities.count;
            locationsX = Array(repeating: 0, count: length)
            locationsY = Array(repeating: 0, count: length)
            sizeW = Array(repeating: 0, count: length)
            sizeH = Array(repeating: 0, count: length)
            forcesX = Array(repeating: 0, count: length)
            forcesY = Array(repeating: 0, count: length)
        }
        for i in 0..<entities.count {
            let ent = entities[i]
            locationsX[i] = ent.x;
            locationsY[i] = ent.y;
            let size = self.layoutContext.getBounds(node: ent)
            sizeW[i] = size.width;
            sizeH[i] = size.height;
        }
    }

    private func saveLocations() {
        if entities.count == 0 {
            return;
        }
        self.operations.removeAll()
        for i in 0..<entities.count {
            // TODO ensure no dynamic layout passes are triggered as a result of
            // storing the positions
            // TODO: check where NaN values originate from
            if locationsX[i].isNaN || locationsY[i].isNaN {
                locationsX[i] = 0;
                locationsY[i] = 0;
            }
            self.operations.append(layoutContext.store.createUpdatePosition(item: entities[i], newPos: CGPoint(x: locationsX[i], y: locationsY[i])))
        }
    }

    /**
     * Scales the current iteration counter based on how long the algorithm has
     * been running for. You can set the MaxTime in maxTimeMS!
     */
    private func setSprIterationsBasedOnTime() {
        if maxTimeMS <= 0 {
            return
        }

        let currentTime = Date();
        let since = currentTime.timeIntervalSince(startTime)
        let fractionComplete = Double(since) / Double(maxTimeMS)
        let currentIteration = Int(fractionComplete * Double(sprIterations))
        if currentIteration > iteration {
            iteration = currentIteration
        }
    }

    /**
     * Performs one iteration based on time.
     *
     * @return <code>true</code> if the maximum number of iterations was not
     *         reached yet, otherwise <code>false</code>.
     */
    func performAnotherNonContinuousIteration() -> Bool {
        self.setSprIterationsBasedOnTime()
        return iteration <= sprIterations
    }

    /**
     * Returns the current iteration.
     *
     * @return The current iteration.
     */
    func getCurrentLayoutStep() -> Int {
        return iteration;
    }

    /**
     * Returns the maximum number of iterations.
     *
     * @return The maximum number of iterations.
     */
    func getTotalNumberOfLayoutSteps() -> Int {
        return sprIterations;
    }

    /**
     * Computes one iteration (forces, positions) and increases the iteration
     * counter.
     */
    func computeOneIteration() {
        self.computeForces()
        self.computePositions()
        let currentBounds = getLayoutBounds()
        self.improveBoundScaleX(currentBounds)
        self.improveBoundScaleY(currentBounds)
        self.moveToCenter(currentBounds)
        self.iteration += 1
    }

    /**
     * Puts vertices in random places, all between (0,0) and (1,1).
     */
    func placeRandomly() {
        if locationsX.count == 0 {
            return
        }

        // If only one node in the data repository, put it in the middle
        if locationsX.count == 1 {
            // If only one node in the data repository, put it in the middle
            locationsX[0] = bounds.origin.x + 0.5 * bounds.width;
            locationsY[0] = bounds.origin.y + 0.5 * bounds.height;
        } else {
            locationsX[0] = bounds.origin.x
            locationsY[0] = bounds.origin.y
            locationsX[1] = bounds.origin.x + bounds.width;
            locationsY[1] = bounds.origin.x + bounds.height;
            for i in 2..<locationsX.count {
                locationsX[i] = bounds.origin.x
                    + CGFloat(drand48()) * bounds.width;
                locationsY[i] = bounds.origin.y
                        + CGFloat(drand48()) * bounds.height;
            }
        }
    }

    /**
     * Computes the force for each node in this SpringLayoutAlgorithm. The
     * computed force will be stored in the data repository
     */
    func computeForces() {

        var forcesX:[[CGFloat]] = Array(repeating: Array(repeating: CGFloat(0), count: self.forcesX.count), count: 2)
        var forcesY:[[CGFloat]] = Array(repeating: Array(repeating: CGFloat(0), count: self.forcesX.count), count: 2)
        var locationsX:[CGFloat] = Array(repeating: CGFloat(0), count: self.forcesX.count)
        var locationsY:[CGFloat] = Array(repeating: CGFloat(0), count: self.forcesY.count)
        
        for j in 0..<2  {
            for i in 0..<self.forcesX.count {
                forcesX[j][i] = 0;
                forcesY[j][i] = 0;
                locationsX[i] = self.locationsX[i];
                locationsY[i] = self.locationsY[i];
            }
        }

        // TODO: Again really really slow!

        for k in 0..<2 {
            for i in 0..<self.locationsX.count {

                for j in i + 1..<self.locationsX.count{
                    let dx = (locationsX[i] - locationsX[j]) / bounds.width / boundsScaleX;
                    let dy = (locationsY[i] - locationsY[j]) / bounds.height / boundsScaleY;
                    var distance_sq = dx * dx + dy * dy;
                    // make sure distance and distance squared not too small
                    distance_sq = max(MIN_DISTANCE * MIN_DISTANCE, distance_sq);
                    let distance = sqrt(distance_sq);

                    // If there are relationships between srcObj and destObj
                    // then decrease force on srcObj (a pull) in direction of
                    // destObj
                    // If no relation between srcObj and destObj then increase
                    // force on srcObj (a push) from direction of destObj.
                    let sumOfWeights = srcDestToSumOfWeights[i][j];

                    var f: CGFloat;
                    if sumOfWeights > 0 {
                        // nodes are pulled towards each other
                        f = -sprStrain * log(distance / sprLength)
                                * sumOfWeights;
                    } else {
                        // nodes are repelled from each other
                        f = sprGravitation / (distance_sq);
                    }
                    let dfx = f * dx / distance;
                    let dfy = f * dy / distance;

                    forcesX[k][i] += dfx;
                    forcesY[k][i] += dfy;

                    forcesX[k][j] -= dfx;
                    forcesY[k][j] -= dfy;
                }
            }

            for i in 0..<entities.count {
                if self.layoutContext.isMovable(entities[i]) {
                    var deltaX = sprMove * forcesX[k][i];
                    var deltaY = sprMove * forcesY[k][i];

                    // constrain movement, so that nodes don't shoot way off to
                    // the
                    // edge
                    let dist = CGFloat(sqrt(deltaX * deltaX + deltaY * deltaY))
                    let maxMovement = 0.2 * sprMove;
                    if dist > maxMovement {
                        deltaX = deltaX * maxMovement / dist;
                        deltaY = deltaY * maxMovement / dist;
                    }

                    locationsX[i] += deltaX * bounds.width * boundsScaleX;
                    locationsY[i] += deltaY * bounds.height * boundsScaleY;
                }
            }

        }
        // // initialize all forces to zero
        for i in 0..<self.entities.count {
            if (forcesX[0][i] * forcesX[1][i] < 0) {
                self.forcesX[i] = 0;
            } else {
                self.forcesX[i] = forcesX[1][i];
            }

            if (forcesY[0][i] * forcesY[1][i] < 0) {
                self.forcesY[i] = 0;
            } else {
                self.forcesY[i] = forcesY[1][i];
            }
        }
    }

    /**
     * Computes the position for each node in this SpringLayoutAlgorithm. The
     * computed position will be stored in the data repository. position =
     * position + sprMove * force
     */
    func computePositions() {
        for i in 0..<self.entities.count {
            if self.layoutContext.isMovable(entities[i]) {
                var deltaX = sprMove * forcesX[i];
                var deltaY = sprMove * forcesY[i];

                // constrain movement, so that nodes don't shoot way off to the
                // edge
                let dist = CGFloat(sqrt(deltaX * deltaX + deltaY * deltaY))
                let maxMovement = 0.2 * sprMove;
                if dist > maxMovement {
                    deltaX = deltaX * maxMovement / dist;
                    deltaY = deltaY * maxMovement / dist;
                }

                locationsX[i] += deltaX * bounds.width * boundsScaleX;
                locationsY[i] += deltaY * bounds.height * boundsScaleY;
            }
        }
    }

    private func getLayoutBounds() -> CGRect {
        var minX: CGFloat
        var maxX: CGFloat
        var minY: CGFloat
        var maxY: CGFloat
        minX = CGFloat.infinity
        minY = CGFloat.infinity
        maxX = -1 * CGFloat.infinity;
        maxY = -1 * CGFloat.infinity;

        for i in 0..<locationsX.count {
            maxX = max(maxX, locationsX[i] + sizeW[i] );
            minX = min(minX, locationsX[i] - sizeW[i] );
            maxY = max(maxY, locationsY[i] + sizeH[i] );
            minY = min(minY, locationsY[i] - sizeH[i] );
        }
        return CGRect(x: minX, y: minY, width: maxX - minX, height: maxY - minY)
    }

    private func improveBoundScaleX(_ currentBounds: CGRect ) {
        let boundaryProportionX = currentBounds.width / bounds.width;
        if boundaryProportionX < 0.9 {
            boundsScaleX *= 1.01;
        } else if boundaryProportionX > 1 {
            if boundsScaleX < 0.01 {
                return;
            }
            boundsScaleX /= 1.01;
        }
    }

    private func improveBoundScaleY(_ currentBounds: CGRect) {
        let boundaryProportionY = currentBounds.height / bounds.height
        if boundaryProportionY < 0.9 {
            boundsScaleY *= 1.01;
        } else if boundaryProportionY > 1 {
            if boundsScaleY < 0.01 {
                return;
            }
            boundsScaleY /= 1.01;
        }
    }

    private func moveToCenter(_ currentBounds: CGRect) {
        let moveX = (currentBounds.origin.x + currentBounds.width / 2) - (bounds.origin.x + bounds.width / 2);
        let moveY = (currentBounds.origin.y + currentBounds.height / 2) - (bounds.origin.y + bounds.height / 2);
        for i in 0..<locationsX.count {
            locationsX[i] -= moveX;
            locationsY[i] -= moveY;
        }
    }
}

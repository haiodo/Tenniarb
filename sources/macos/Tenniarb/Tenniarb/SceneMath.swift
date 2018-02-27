//
//  SceneMath.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 14/08/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation


/**
 Calc point to cross two lines.
 */
func crossLine( _ p1: CGPoint, _ p2: CGPoint, _ p3: CGPoint, _ p4: CGPoint) -> CGPoint? {
    let d = (p1.x - p2.x) * (p4.y - p3.y) - (p1.y - p2.y) * (p4.x - p3.x)
    let da = (p1.x - p3.x) * (p4.y - p3.y) - (p1.y - p3.y) * (p4.x - p3.x)
    let db = (p1.x - p2.x) * (p1.y - p3.y) - (p1.y - p2.y) * (p1.x - p3.x)
    
    let ta = da / d;
    let tb = db / d;
    
    if ta >= 0 && ta <= 1 && tb >= 0 && tb <= 1
    {
        let dx = p1.x + ta * (p2.x - p1.x)
        let dy = p1.y + ta * (p2.y - p1.y)
        
        return CGPoint(x: dx, y: dy)
    }
    
    return nil
}
func crossBox( _ p1:CGPoint, _ p2: CGPoint, _ rect: CGRect)-> CGPoint? {
    let op = rect.origin
    
    //0,0 -> 1,0
    if let cp = crossLine( p1, p2, CGPoint(x: op.x, y: op.y), CGPoint( x: op.x + rect.width, y: op.y) ) {
        return cp
    }
    
    // 0,0 -> 0, 1
    if let cp = crossLine( p1, p2, CGPoint(x: op.x, y: op.y), CGPoint( x: op.x, y: op.y + rect.height) ) {
        return cp
    }
    
    // 0,1 -> 1, 1
    if let cp = crossLine( p1, p2, CGPoint(x: op.x, y: op.y + rect.height), CGPoint( x: op.x + rect.width, y: op.y + rect.height) ) {
        return cp
    }
    // 1,0 -> 1,1
    if let cp = crossLine( p1, p2, CGPoint(x: op.x + rect.width, y: op.y), CGPoint( x: op.x + rect.width, y: op.y + rect.height) ) {
        return cp
    }
    return nil
}

/*
 Will return -1 in case point is not in box of p1,p2.
 */
func crossPointLine(_ p1: CGPoint, _ p2: CGPoint, _ p: CGPoint) -> CGFloat {
    var r = CGRect(origin: CGPoint(x: min(p1.x, p2.x), y: min(p1.y, p2.y)), size: CGSize(width: abs(p1.x-p2.x), height: abs(p1.y-p2.y) ))
    if r.height < 10 {
        r = CGRect(origin: CGPoint(x: r.origin.x, y: r.origin.y-5), size: CGSize(width: r.width, height: r.height+10 ))
    }
    
    if !r.contains(p) {
        // Point not in boundaries of line
        return -1
    }
    if p1.x == p2.x && p1.y == p2.y {
        return -1.0
    }
    
    let A = p2.y - p1.y
    let B = p1.x - p2.x
    let C = -( p1.x*A + p1.y * B )
    
    let d = abs(A*p.x+B*p.y+C)/sqrt(A*A+B*B)
    return d
}

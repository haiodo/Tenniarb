//
//  SceneMath.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 14/08/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
func crossPointLine(_ p1: CGPoint, _ p2: CGPoint, _ p: CGPoint) -> Bool {
    
    let dist1 = sqrt((p1.x-p.x)*(p1.x-p.x) + (p1.y-p.y)*(p1.y-p.y))
    if dist1 < 7 {
        return true
    }
    
    let dist2 = sqrt((p2.x-p.x)*(p2.x-p.x) + (p2.y-p.y)*(p2.y-p.y))
    if dist2 < 7 {
        return true
    }

    if p1.x == p2.x && p1.y == p2.y {
        return false
    }
    
    let A = p2.y - p1.y
    let B = p1.x - p2.x
    let C = -( p1.x*A + p1.y * B )
    
    let d = abs(A*p.x+B*p.y+C)/sqrt(A*A+B*B)
    if d >= 0 && d < 7 {
        
        // We need to check if in boundaries of line.
        var r = CGRect(origin: CGPoint(x: min(p1.x, p2.x), y: min(p1.y, p2.y)), size: CGSize(width: abs(p1.x-p2.x), height: abs(p1.y-p2.y) ))
        
        if r.height < 5 {
            r = CGRect(origin: CGPoint(x: r.origin.x, y: r.origin.y - 2.5), size: CGSize(width: r.width, height: r.height + 5 ))
        }
        if r.width < 5 {
            r = CGRect(origin: CGPoint(x: r.origin.x - 2.5, y: r.origin.y), size: CGSize(width: r.width + 5, height: r.height))
        }
        
        let lineP =  CGPoint(x: (B*(B*p.x-A*p.y) - A*C) / (A*A+B*B),
                             y: (A*(-1*B*p.x+A*p.y) - B*C) / (A*A+B*B))
        
        if !r.contains(lineP) {
            // Point not in boundaries of line
            return false
        }
        
        return true
    }
    return false
}

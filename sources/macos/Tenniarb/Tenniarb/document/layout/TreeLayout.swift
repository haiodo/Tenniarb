//
//  TreeLayout.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation

enum TreeDirection {
    case TopDown
    case BottomUp
    case LeftRight
    case RightLeft
}

public class TreeLayout: LayoutAlgorithm {
    private var direction: TreeDirection = .TopDown
    private var bounds: CGRect = CGRect(x: 0, y: 0, width: 0, height: 0)
    private var leftSize: CGFloat = 0
    private var layerSize: CGFloat = 0
    
    
    public func apply(context: LayoutContext, clean: Bool) {
        
    }
    
}

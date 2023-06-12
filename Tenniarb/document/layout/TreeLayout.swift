//
//  TreeLayout.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 15/01/2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
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
    
    
    public func apply(context: LayoutContext, clean: Bool) -> [ElementOperation] {
        return []
    }
    
}

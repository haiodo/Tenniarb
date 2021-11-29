//
//  mainView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 14.11.2021.
//  Copyright © 2021 Andrey Sobolev. All rights reserved.
//

import SwiftUI

struct NodeOutlineGroup: View {
    let element: Element
    @State var isExpanded: Bool = true

    var body: some View {
        if !element.elements.isEmpty {
            DisclosureGroup(
                isExpanded: $isExpanded,
                content: {
                    if isExpanded {
                        ForEach(element.elements) { childNode in
                            NodeOutlineGroup(element:childNode, isExpanded: isExpanded)
                        }
                    }
                },
                label: { Text(element.name) })
        } else {
            Text(element.name)
        }
    }
}

struct mainView: View {
    let element: Element
    var body: some View {
        NavigationView {
            List {
                ForEach(element.elements) { childNode in
                    NodeOutlineGroup(element:childNode, isExpanded: true)
                }
            }
        }
    }
}

//struct mainView_Previews: PreviewProvider {
//    static var previews: some View {
//        Group {
//            mainView(element: loadTestDocument())
//        }
//    }
//}


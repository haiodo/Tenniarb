//
//  HelpController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 10.07.2019.
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

import Cocoa
import Foundation
import WebKit

class HelpController: NSViewController {
    var webView: WKWebView!

    override func loadView() {
        // Build the view programmatically so this controller doesn't rely on the storyboard.
        let root = NSView(frame: NSRect(x: 0, y: 0, width: 1024, height: 768))
        root.wantsLayer = true

        let config = WKWebViewConfiguration()
        let wk = WKWebView(frame: .zero, configuration: config)
        wk.translatesAutoresizingMaskIntoConstraints = false

        root.addSubview(wk)
        NSLayoutConstraint.activate([
            wk.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            wk.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            wk.topAnchor.constraint(equalTo: root.topAnchor),
            wk.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])

        self.view = root
        self.webView = wk
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        if let filePath = Bundle.main.path(forResource: "readme", ofType: "html") {
            let request = URLRequest(url: URL(fileURLWithPath: filePath))
            webView.load(request)
        }
    }
}

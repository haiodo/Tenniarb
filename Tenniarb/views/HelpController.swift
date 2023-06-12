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

import Foundation
import Cocoa
import WebKit

class HelpController: NSViewController  {
    @IBOutlet weak var webView: WKWebView!
    override func viewDidLoad() {
        super.viewDidLoad()
        let filePath = Bundle.main.path(forResource: "readme", ofType: "html")
        let request = URLRequest(url: URL(fileURLWithPath: filePath!))
        webView.load(request)
    }
}

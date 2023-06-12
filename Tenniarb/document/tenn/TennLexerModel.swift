//
//  TennLexerModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 23.08.2019.
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

public enum TennTokenType {
    case invalid
    case eof
    case symbol
    case floatLit
    case intLit
    case stringLit
    case curlyLe
    case curlyRi
    case comma
    case colon
    case semiColon
    case expression         // $(expression)
    case expressionBlock    // ${expression block}
    case markdownLit // %{}
    case imageData  // @"base64" - encoded png image
}

public class TennToken {
    public let type: TennTokenType
    public let literal: String
    public let line: Int
    public let col: Int
    public let pos: Int
    public let size: Int
    
    
    @inlinable
    init( type: TennTokenType, literal: String, line: Int = 0, col:Int = 0, pos:Int = 0, size:Int = 0) {
        self.type = type
        self.literal = literal
        self.line = line
        self.col = col
        self.pos = pos
        self.size = size
    }
}

extension TennToken: Hashable {
    public static func == (lhs: TennToken, rhs: TennToken) -> Bool {
        return lhs.type == rhs.type && lhs.literal == rhs.literal && lhs.line == rhs.line && lhs.col == lhs.col && lhs.pos == rhs.pos && lhs.size == rhs.size;
    }
    public func hash(into hasher: inout Hasher) {
        hasher.combine(self.type)
        hasher.combine(self.literal)
        hasher.combine(self.col)
        hasher.combine(self.pos)
        hasher.combine(self.size)
    }
}

public enum LexerError {
    case EndOfLineReadString
    case EndOfExpressionReadError
    case UTF8Error
}

public protocol TennLexerProtocol {
    func getToken() -> TennToken?
    func revert(tok: TennToken)
    
    var errorHandler: ((_ error: LexerError, _ startPos:Int, _ pos: Int ) -> Void)? {
        set
        get
    }
}

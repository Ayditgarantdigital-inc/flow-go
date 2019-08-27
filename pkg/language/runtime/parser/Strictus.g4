grammar Strictus;

// for handling optional semicolons between statement, see also `eos` rule

// NOTE: unusued builder variable, to avoid unused import error because
//    import will also be added to visitor code
@parser::header {
    import "strings"
    var _ = strings.Builder{}
}

@parser::members {
    // Returns true if on the current index of the parser's
    // token stream a token exists on the Hidden channel which
    // either is a line terminator, or is a multi line comment that
    // contains a line terminator.
    func (p *StrictusParser) lineTerminatorAhead() bool {
        // Get the token ahead of the current index.
        possibleIndexEosToken := p.GetCurrentToken().GetTokenIndex() - 1
        ahead := p.GetTokenStream().Get(possibleIndexEosToken)

        if ahead.GetChannel() != antlr.LexerHidden {
            // We're only interested in tokens on the HIDDEN channel.
            return true
        }

        if ahead.GetTokenType() == StrictusParserTerminator {
            // There is definitely a line terminator ahead.
            return true
        }

        if ahead.GetTokenType() == StrictusParserWS {
            // Get the token ahead of the current whitespaces.
            possibleIndexEosToken = p.GetCurrentToken().GetTokenIndex() - 2
            ahead = p.GetTokenStream().Get(possibleIndexEosToken)
        }

        // Get the token's text and type.
        text := ahead.GetText()
        _type := ahead.GetTokenType()

        // Check if the token is, or contains a line terminator.
        return (_type == StrictusParserBlockComment && (strings.Contains(text, "\r") || strings.Contains(text, "\n"))) ||
            (_type == StrictusParserTerminator)
    }
}

program
    : (declaration ';'?)* EOF
    ;

declaration
    : structureDeclaration
    | interfaceDeclaration
    | functionDeclaration[true]
    | variableDeclaration
    | importDeclaration
    ;

importDeclaration
    : Import (identifier (',' identifier)* From)? (stringLiteral | HexadecimalLiteral)
    ;

access
    : /* Not specified */
    | Pub
    | PubSet
    ;

structureDeclaration
    : Struct identifier conformances '{'
        field*
        initializer[true]?
        functionDeclaration[true]*
      '}'
    ;

conformances
    : (':' identifier (',' identifier)*)?
    ;

variableKind
    : Let
    | Var
    ;

field
    : access variableKind? identifier ':' fullType
    ;

interfaceDeclaration
    : Interface identifier '{' field* initializer[false]? functionDeclaration[false]* '}'
    ;

// NOTE: allow any identifier in parser, then check identifier
// is `init` in semantic analysis to provide better error
//
initializer[bool functionBlockRequired]
    : identifier parameterList
      // only optional if parameter functionBlockRequired is false
      b=functionBlock? { !$functionBlockRequired || $ctx.b != nil }?
    ;

functionDeclaration[bool functionBlockRequired]
    : access Fun identifier parameterList (':' returnType=fullType)?
      // only optional if parameter functionBlockRequired is false
      b=functionBlock? { !$functionBlockRequired || $ctx.b != nil }?
    ;

parameterList
    : '(' (parameter (',' parameter)*)? ')'
    ;

parameter
    : (argumentLabel=identifier)? parameterName=identifier ':' fullType
    ;

fullType
    : baseType typeIndex* (optionals+=(Optional|NilCoalescing))*
    ;

typeIndex
    : '[' (DecimalLiteral|fullType)? ']'
    ;

baseType
    : identifier
    | functionType
    ;

functionType
    : '(' '(' (parameterTypes+=fullType (',' parameterTypes+=fullType)*)? ')' ':' returnType=fullType ')'
    ;

block
    : '{' statements '}'
    ;

functionBlock
    : '{' preConditions? postConditions? statements '}'
    ;

preConditions
    : Pre '{' conditions '}'
    ;

postConditions
    : Post '{' conditions '}'
    ;

conditions
    : (condition eos)*
    ;

condition
    : test=expression (':' message=expression)?
    ;

statements
    : (statement eos)*
    ;

statement
    : returnStatement
    | breakStatement
    | continueStatement
    | ifStatement
    | whileStatement
    // NOTE: allow all declarations, even structures, in parser,
    // then check identifier declaration is variable/constant or function
    // in semantic analysis to provide better error
    | declaration
    | assignment
    | expression
    ;

returnStatement
    : Return expression?
    ;

breakStatement
    : Break
    ;

continueStatement
    : Continue
    ;

ifStatement
    : If
      (testExpression=expression | testDeclaration=variableDeclaration)
      then=block
      (Else (ifStatement | alt=block))?
    ;

whileStatement
    : While expression block
    ;

variableDeclaration
    : variableKind identifier (':' fullType)? '=' expression
    ;

assignment
    : identifier expressionAccess* '=' expression
    ;

expression
    : conditionalExpression
    ;

conditionalExpression
    : <assoc=right> orExpression ('?' then=expression ':' alt=expression)?
    ;

orExpression
    : andExpression
    | orExpression '||' andExpression
    ;

andExpression
    : equalityExpression
    | andExpression '&&' equalityExpression
    ;

equalityExpression
    : relationalExpression
    | equalityExpression equalityOp relationalExpression
    ;

relationalExpression
    : nilCoalescingExpression
    | relationalExpression relationalOp nilCoalescingExpression
    ;

nilCoalescingExpression
    // NOTE: right associative
    : failableDowncastingExpression (NilCoalescing nilCoalescingExpression)?
    ;

failableDowncastingExpression
    : additiveExpression
    | failableDowncastingExpression FailableDowncasting fullType
    ;

additiveExpression
    : multiplicativeExpression
    | additiveExpression additiveOp multiplicativeExpression
    ;

multiplicativeExpression
    : unaryExpression
    | multiplicativeExpression multiplicativeOp unaryExpression
    ;

unaryExpression
    : primaryExpression
    | unaryOp+ unaryExpression
    ;

primaryExpression
    : primaryExpressionStart primaryExpressionSuffix*
    ;

primaryExpressionSuffix
    : expressionAccess
    | invocation
    ;

equalityOp
    : Equal
    | Unequal
    ;

Equal : '==' ;
Unequal : '!=' ;

relationalOp
    : Less
    | Greater
    | LessEqual
    | GreaterEqual
    ;

Less : '<' ;
Greater : '>' ;
LessEqual : '<=' ;
GreaterEqual : '>=' ;

additiveOp
    : Plus
    | Minus
    ;

Plus : '+' ;
Minus : '-' ;

multiplicativeOp
    : Mul
    | Div
    | Mod
    ;

Mul : '*' ;
Div : '/' ;
Mod : '%' ;


unaryOp
    : Minus
    | Negate
    ;

Negate : '!' ;

Optional : '?' ;

NilCoalescing : '??' ;

FailableDowncasting : 'as?' ;

primaryExpressionStart
    : identifier                                                  # IdentifierExpression
    | literal                                                     # LiteralExpression
    | Fun parameterList (':' returnType=fullType)? functionBlock  # FunctionExpression
    | '(' expression ')'                                          # NestedExpression
    ;

expressionAccess
    : memberAccess
    | bracketExpression
    ;

memberAccess
    : '.' identifier
    ;

bracketExpression
    : '[' expression ']'
    ;

invocation
    : '(' (argument (',' argument)*)? ')'
    ;

argument
    : (identifier ':')? expression
    ;

literal
    : integerLiteral
    | booleanLiteral
    | arrayLiteral
    | dictionaryLiteral
    | stringLiteral
    | nilLiteral
    ;

booleanLiteral
    : True
    | False
    ;

nilLiteral
    : Nil
    ;

stringLiteral
    : StringLiteral
    ;

integerLiteral
    : DecimalLiteral        # DecimalLiteral
    | BinaryLiteral         # BinaryLiteral
    | OctalLiteral          # OctalLiteral
    | HexadecimalLiteral    # HexadecimalLiteral
    | InvalidNumberLiteral  # InvalidNumberLiteral
    ;

arrayLiteral
    : '[' ( expression (',' expression)* )? ']'
    ;

dictionaryLiteral
    : '{' ( dictionaryEntry (',' dictionaryEntry)* )? '}'
    ;

dictionaryEntry
    : key=expression ':' value=expression
    ;

OpenParen: '(' ;
CloseParen: ')' ;

Transaction : 'transaction' ;

Struct : 'struct' ;

Interface : 'interface' ;

Fun : 'fun' ;

Pre : 'pre' ;
Post : 'post' ;

Pub : 'pub' ;
PubSet : 'pub(set)' ;

Return : 'return' ;

Break : 'break' ;
Continue : 'continue' ;

Let : 'let' ;
Var : 'var' ;

If : 'if' ;
Else : 'else' ;

While : 'while' ;

True : 'true' ;
False : 'false' ;

Nil : 'nil' ;

Import : 'import' ;
From : 'from' ;

identifier
    : Identifier
    | From
    ;

Identifier
    : IdentifierHead IdentifierCharacter*
    ;

fragment IdentifierHead
    : [a-zA-Z]
    |  '_'
    ;

fragment IdentifierCharacter
    : [0-9]
    | IdentifierHead
    ;


DecimalLiteral
    // NOTE: allows trailing underscores, but the parser checks underscores
    // only occur inside, to provide better syntax errors
    : [0-9] [0-9_]*
    ;


BinaryLiteral
    // NOTE: allows underscores anywhere after prefix, but the parser checks underscores
    // only occur inside, to provide better syntax errors
    : '0b' [01_]+
    ;


OctalLiteral
    // NOTE: allows underscores anywhere after prefix, but the parser checks underscores
    // only occur inside, to provide better syntax errors
    : '0o' [0-7_]+
    ;

HexadecimalLiteral
    // NOTE: allows underscores anywhere after prefix, but the parser checks underscores
    // only occur inside, to provide better syntax errors
    : '0x' [0-9a-fA-F_]+
    ;

// NOTE: invalid literal, to provide better syntax errors
InvalidNumberLiteral
    : '0' [a-zA-Z] [0-9a-zA-Z_]*
    ;

StringLiteral
    : '"' QuotedText* '"'
    ;

fragment QuotedText
    : EscapedCharacter
    | ~["\n\r\\]
    ;

fragment EscapedCharacter
    : '\\' [0\\tnr"']
    // NOTE: allow arbitrary length in parser, but check length in semantic analysis
    | '\\u' '{' HexadecimalDigit+ '}'
    ;

fragment HexadecimalDigit : [0-9a-fA-F] ;


WS
    : [ \t\u000B\u000C\u0000]+ -> channel(HIDDEN)
    ;

Terminator
    : [\r\n]+ -> channel(HIDDEN)
    ;

BlockComment
    : '/*' (BlockComment|.)*? '*/'	-> channel(HIDDEN) // nesting comments allowed
    ;

LineComment
    : '//' ~[\r\n]* -> channel(HIDDEN)
    ;

eos
    : ';'
    | EOF
    | {p.lineTerminatorAhead()}?
    | {p.GetTokenStream().LT(1).GetText() == "}"}?
    ;

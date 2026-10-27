"
I test Add.
"
Class {
	#name : #AddTest,
	#superclass : #TestCase,
	#category : #'Add-Tests'
}

{ #category : #tests }
AddTest >> testAdd [

	self assert: (Add add: 2 to: 3) equals: 5
]

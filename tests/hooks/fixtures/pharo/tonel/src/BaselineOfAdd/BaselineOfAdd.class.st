"
I load the Add package and its tests.
"
Class {
	#name : #BaselineOfAdd,
	#superclass : #BaselineOf,
	#category : #BaselineOfAdd
}

{ #category : #baselines }
BaselineOfAdd >> baseline: spec [

	<baseline>
	spec for: #common do: [
		spec
			package: 'Add';
			package: 'Add-Tests' with: [ spec requires: #( 'Add' ) ] ]
]

Given('ok') { }
Given('this is pending') { pending('not yet') }
Given(/^this matches twice$/) { }
Given(/^this matches (twice|thrice)$/) { |_| }
Given('it fails') { raise 'boom' }

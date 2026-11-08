PNG = ["89504e470d0a1a0a0000000d4948445200000001000000010806000000" "1f15c4890000000d49444154789c6360000002000154a24f5d0000000049454e44ae426082"].pack('H*')
Given('I attach text, png, json and log') do
  attach('hello <b>text</b>', 'text/plain')
  attach(PNG, 'image/png')
  attach('{"a":1}', 'application/json')
  log('a log line')
end
Given('it fails') { raise 'boom' }

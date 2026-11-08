Given(/^I attach images (\d+) to (\d+)$/) do |a, b|
  (a.to_i..b.to_i).each { |i| attach(File.join('blobs', format('img%02d.png', i)), 'image/png') }
end
Given(/^I attach the big text$/) do
  attach(File.read('blobs/big.txt'), 'text/plain')
end

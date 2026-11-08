$attempts = Hash.new(0)
Given('the shop is open') { log('shop opened'); attach('{"k":"v"}', 'application/json') }
When('I pay') { log("paying attempt") }
Then('the payment succeeds') do
  $attempts[:flaky] += 1
  raise 'transient gateway error' if $attempts[:flaky] < 2
end
Then('the payment is declined') { raise 'card declined' }
When('I apply code {word}') { |c| @c = c }
Then('the total is {int}') { |t| attach("total=#{t}", 'text/plain') }

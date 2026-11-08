Given('the store {string} is open') { |n| @store = n; @cart = 0 }
Given('I am logged in as {string}') { |u| @user = u }
When('I add {int} items') { |n| @cart = (@cart || 0) + n }
Then('the cart has {int} items') do |n|
  raise RSpec::Expectations::ExpectationNotMetError, "expected cart to have #{n} items but had #{@cart}" if defined?(RSpec) && @cart != n
  raise "expected cart to have #{n} items but had #{@cart}" unless @cart == n
end
When('I apply coupons:') do |table|
  @pct = table.hashes.sum { |h| h['pct'].to_i }
end
Then('the total is {int}') do |t|
  got = 100 - @pct
  raise "total #{got} != #{t}" unless got == t
end
When('I apply an invalid coupon {string}') do |c|
  raise ArgumentError, "\e[31mCoupon #{c} rejected\e[0m\n  \e[1mexpected\e[0m: a valid coupon\n  \e[1mgot\e[0m:      \e[33m#{c}\e[0m <script>alert(1)</script> & 'single' \"double\"\n" + ('long-diff-' * 300)
end
Then('the receipt is:') do |doc|
  JSON.parse(doc) rescue raise("receipt is not JSON: #{doc}")
end
When('the payment gateway responds for {string}') do |key|
  if plan_fail?(key) || (ENV['BROWSER'] == 'firefox' && key == 'gw-firefox')
    raise IOError, "Net::ReadTimeout: gateway #{key} timed out after 30s (browser=#{ENV['BROWSER']})"
  end
end
When('I do the composite action') do
  step 'I add 2 items'
  step 'the cart has 2 items'
end
When('I do the failing composite action') do
  step 'I add 2 items'
  step 'the cart has 5 items'
end
Then('the deep call fails with {string}') { |m| Deep.level_a(m) }
Then('the feature is pending') { pending('waiting on JIRA-123: "pending" with <msg> & ünï') }
Then(/^I am ambig(uous)?$/) { |_| nil }
Then('I am ambiguous') { nil }
Then('I take a screenshot as {string}') do |kind|
  f = { 'png' => 'shot.png', 'jpg' => 'shot.jpg', 'svg' => 'diagram.svg', 'pdf' => 'report.pdf', 'json' => 'api.json', 'html' => 'page.html' }.fetch(kind)
  mt = { 'png' => 'image/png', 'jpg' => 'image/jpeg', 'svg' => 'image/svg+xml', 'pdf' => 'application/pdf', 'json' => 'application/json', 'html' => 'text/html' }.fetch(kind)
  attach(File.join(BLOBS, f), mt)
end
Then('I attach a text log') { attach("plain log ünï 🎉\nsecond line", 'text/plain') }
Then('the step {string} passes') { |_| nil }
Then('the step {string} fails') { |s| raise "step #{s} failed: \e[31mboom\e[0m" }
# French
Soit('le magasin {string} est ouvert') { |n| @store = n; @cart = 0 }
Quand('j\'ajoute {int} articles') { |n| @cart = (@cart || 0) + n }
Alors('le panier contient {int} articles') { |n| raise "panier attendu #{n}, obtenu #{@cart} — élève à l'école" unless @cart == n }

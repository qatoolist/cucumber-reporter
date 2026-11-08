Around('@ar') do |scenario, block|
  block.call
  raise "around after flaky #{scenario.name}" if scenario.name == 'Wrapped two' && bump('around_w2') == 1
end

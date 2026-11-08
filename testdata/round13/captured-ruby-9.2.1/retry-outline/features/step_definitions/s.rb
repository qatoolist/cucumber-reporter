$n = Hash.new(0)
Given(/^row (\d+) behaves (\w+)$/) do |row, how|
  $n[row]+=1
  case how
  when 'flaky' then raise "row #{row} flaky" if $n[row]==1
  when 'broken' then raise "row #{row} broken"
  when 'pending_after' then raise "row #{row} first" if $n[row]==1; pending('row pending')
  end
end

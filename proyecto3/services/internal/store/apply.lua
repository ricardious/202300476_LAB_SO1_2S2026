-- Una transacción evita dobles conteos al reenviar un mensaje confirmado tarde.
local prefix = KEYS[1]
if redis.call('EXISTS', prefix .. 'seen:' .. ARGV[1]) == 1 then return 0 end
local planes, ships = tonumber(ARGV[3]), tonumber(ARGV[4])
-- Preflight de tipos antes de escribir: Lua de Redis no revierte errores parciales.
for _, name in ipairs({'stats','planes:hist','ships:hist','latest:' .. ARGV[2]}) do
 local t=redis.call('TYPE',prefix .. name).ok
 if t~='none' and t~='hash' then return redis.error_reply('invalid storage type') end
end
for _, name in ipairs({'planes:rank','ships:rank'}) do
 local t=redis.call('TYPE',prefix .. name).ok
 if t~='none' and t~='zset' then return redis.error_reply('invalid storage type') end
end
local t=redis.call('TYPE',prefix .. 'timeline').ok
if t~='none' and t~='stream' then return redis.error_reply('invalid storage type') end
local count=tonumber(redis.call('HGET',prefix .. 'stats','total') or '0')
if count==0 then
 redis.call('HSET',prefix .. 'stats','planes_min',planes,'planes_max',planes,'ships_min',ships,'ships_max',ships)
else
 for _, m in ipairs({'planes','ships'}) do
  local v=m=='planes' and planes or ships
  if v<tonumber(redis.call('HGET',prefix .. 'stats',m .. '_min')) then redis.call('HSET',prefix .. 'stats',m .. '_min',v) end
  if v>tonumber(redis.call('HGET',prefix .. 'stats',m .. '_max')) then redis.call('HSET',prefix .. 'stats',m .. '_max',v) end
 end
end
redis.call('HINCRBY',prefix .. 'stats','total',1)
redis.call('HINCRBY',prefix .. 'stats','country:' .. ARGV[2],1)
redis.call('HINCRBY',prefix .. 'planes:hist',planes,1)
redis.call('HINCRBY',prefix .. 'ships:hist',ships,1)
redis.call('ZINCRBY',prefix .. 'planes:rank',planes,ARGV[2])
redis.call('ZINCRBY',prefix .. 'ships:rank',ships,ARGV[2])
local previous=redis.call('HGET',prefix .. 'latest:' .. ARGV[2],'epoch')
if not previous or tonumber(ARGV[6])>=tonumber(previous) then
 redis.call('HSET',prefix .. 'latest:' .. ARGV[2],'planes',planes,'ships',ships,'timestamp',ARGV[5],'epoch',ARGV[6])
end
redis.call('XADD',prefix .. 'timeline','MAXLEN','=',10000,'*','country',ARGV[2],'planes',planes,'ships',ships,'timestamp',ARGV[5])
redis.call('SET',prefix .. 'seen:' .. ARGV[1],'1','EX',86400)
return 1

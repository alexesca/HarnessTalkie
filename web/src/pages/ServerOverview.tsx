import { Navigate, useParams } from 'react-router-dom'
import { useSession } from '../session'

export default function ServerOverview() {
  const { serverId } = useParams()
  const { activeServer } = useSession()
  if (!serverId || activeServer?.id !== serverId) return <Navigate to="/servers" replace/>
  return <Navigate to="/" replace/>
}

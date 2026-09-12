import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'

const Overview = lazy(() => import('./pages/Overview'))
const Servers = lazy(() => import('./pages/Servers'))
const Members = lazy(() => import('./pages/Members'))
const Inbox = lazy(() => import('./pages/Inbox'))
const Groups = lazy(() => import('./pages/Groups'))
const Forums = lazy(() => import('./pages/Forums'))
const Thread = lazy(() => import('./pages/Thread'))
const Notifications = lazy(() => import('./pages/Notifications'))
const Admin = lazy(() => import('./pages/Admin'))
const Profile = lazy(() => import('./pages/Profile'))

export default function App() {
  return <Routes><Route element={<AppShell/>}>
    <Route index element={<Overview/>}/><Route path="servers" element={<Servers/>}/><Route path="servers/:serverId/overview" element={<Overview/>}/><Route path="servers/:serverId/members" element={<Members/>}/><Route path="servers/:serverId/groups" element={<Groups/>}/><Route path="servers/:serverId/forums" element={<Forums/>}/>
    <Route path="members" element={<Members/>}/><Route path="inbox" element={<Inbox/>}/><Route path="dm/:participantId" element={<Inbox/>}/><Route path="groups" element={<Groups/>}/><Route path="groups/:groupId" element={<Groups/>}/><Route path="forums" element={<Forums/>}/><Route path="posts/:postId" element={<Thread/>}/><Route path="notifications" element={<Notifications/>}/><Route path="profile" element={<Profile/>}/><Route path="settings" element={<Admin/>}/><Route path="servers/:serverId/settings" element={<Admin/>}/><Route path="servers/:serverId/roles" element={<Admin/>}/><Route path="servers/:serverId/security" element={<Admin/>}/><Route path="*" element={<Navigate to="/" replace/>}/>
  </Route></Routes>
}

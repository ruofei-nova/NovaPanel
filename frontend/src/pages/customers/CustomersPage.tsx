import { useCallback, useEffect, useState } from 'react';
import {
  Button, Card, Form, Input, Layout, Modal, Popconfirm, Space, Switch, Table, Tag, Typography,
} from 'antd';
import { CopyOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons';

import AppSidebar from '@/layouts/AppSidebar';
import { ClipboardManager, HttpUtil } from '@/utils';

interface Customer {
  id: number;
  username: string;
  role: 'customer';
  enabled: boolean;
}

interface CredentialResult {
  user?: Customer;
  password: string;
}

export default function CustomersPage() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [credential, setCredential] = useState<{ username: string; password: string } | null>(null);
  const [form] = Form.useForm<{ username: string; password?: string }>();

  const load = useCallback(async () => {
    const msg = await HttpUtil.get<Customer[]>('/panel/api/customers/list');
    if (msg?.success && msg.obj) setCustomers(msg.obj);
  }, []);

  useEffect(() => { void load(); }, [load]);

  async function createCustomer() {
    const values = await form.validateFields();
    const msg = await HttpUtil.post<CredentialResult>('/panel/api/customers/create', values);
    if (!msg?.success || !msg.obj) return;
    setCreateOpen(false);
    form.resetFields();
    setCredential({ username: msg.obj.user?.username || values.username, password: msg.obj.password });
    await load();
  }

  async function resetPassword(customer: Customer) {
    const msg = await HttpUtil.post<{ password: string }>(`/panel/api/customers/${customer.id}/reset-password`);
    if (msg?.success && msg.obj) {
      setCredential({ username: customer.username, password: msg.obj.password });
    }
  }

  return (
    <Layout className="page-shell">
      <AppSidebar />
      <Layout.Content className="content-area">
        <Card
          title="客户账号"
          extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>创建客户</Button>}
        >
          <Table
            rowKey="id"
            dataSource={customers}
            pagination={false}
            columns={[
              { title: 'ID', dataIndex: 'id', width: 80 },
              { title: '用户名', dataIndex: 'username' },
              {
                title: '状态',
                render: (_, row) => (
                  <Switch
                    checked={row.enabled}
                    checkedChildren="启用"
                    unCheckedChildren="停用"
                    onChange={async (enabled) => {
                      await HttpUtil.post(`/panel/api/customers/${row.id}/enabled`, { enabled });
                      await load();
                    }}
                  />
                ),
              },
              {
                title: '角色',
                render: () => <Tag color="cyan">客户</Tag>,
              },
              {
                title: '操作',
                render: (_, row) => (
                  <Space>
                    <Button icon={<KeyOutlined />} onClick={() => resetPassword(row)}>重置密码</Button>
                    <Popconfirm
                      title="删除客户账号？节点会解除绑定，但不会删除。"
                      onConfirm={async () => {
                        await HttpUtil.post(`/panel/api/customers/${row.id}/delete`);
                        await load();
                      }}
                    >
                      <Button danger>删除</Button>
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </Card>

        <Modal
          open={createOpen}
          title="创建客户账号"
          okText="生成账号"
          cancelText="取消"
          onOk={createCustomer}
          onCancel={() => setCreateOpen(false)}
        >
          <Form form={form} layout="vertical">
            <Form.Item name="username" label="客户用户名" rules={[{ required: true }]}>
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item
              name="password"
              label="自定义密码（留空则安全随机生成）"
              rules={[{ min: 12, message: '密码至少 12 位' }]}
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          open={!!credential}
          title="客户登录凭据（仅显示这一次）"
          footer={<Button type="primary" onClick={() => setCredential(null)}>我已安全保存</Button>}
          closable={false}
        >
          <Typography.Paragraph>用户名：<Typography.Text code>{credential?.username}</Typography.Text></Typography.Paragraph>
          <Typography.Paragraph>
            密码：<Typography.Text code>{credential?.password}</Typography.Text>
          </Typography.Paragraph>
          <Button
            icon={<CopyOutlined />}
            onClick={() => ClipboardManager.copyText(`用户名: ${credential?.username}\n密码: ${credential?.password}`)}
          >
            复制凭据
          </Button>
        </Modal>
      </Layout.Content>
    </Layout>
  );
}
